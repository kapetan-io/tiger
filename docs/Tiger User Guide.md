# Tiger User Guide

This page is the reference for running tiger: each command, the files a repository commits for
it, the pull request workflow for `tiger changed`, and the settings the golangci-lint plugin
needs. The [Explainer](Tiger%20Explainer.md) says what tiger is for and why, and defines the
concepts this page uses. The [Rule Reference](Tiger%20Rule%20Reference.md) covers each rule.

## Install

```
go install github.com/kapetan-io/tiger/cmd/tiger@latest
```

Tiger has no released tags yet, so `@latest` installs the newest commit on `main`. Pin a commit
(`@<commit sha>`) when a CI job must not change under you.

Tiger reads the export data that the `go` command writes, so install a current Tiger when you upgrade Go.
A Tiger that predates this note fails every package load under Go 1.27.2 and later with `export data version 5 is
greater than maximum supported version 4`.

## Commands

`tiger -h` lists every command, and `tiger <command> -h` lists a command's flags, except
`tiger changed -h`, which prints only its usage line.

```
tiger check ./...               # run the registered custom-rule analyzers
tiger check --show-facts ./...  # also print computed facts in pin syntax
tiger budget ./...              # report packages over their budget rows
tiger budget --write ./...      # lower tiger.budget.yaml to the current counts
tiger golangci                  # audit .golangci.yml against the auto-rule baseline
tiger golangci --init           # generate the baseline config for a new project
tiger golangci --print          # write the baseline to stdout for hand merging
tiger pattern                   # list every pattern
tiger pattern shutdown-loop     # print one pattern: its code, tests and reviewer checklist
tiger pattern --rule TS-C02     # list the patterns behind a rule code
tiger pin Drain ./...           # freeze a function's computed facts into //tiger: pins
tiger pin --dry-run Drain ./... # print what tiger pin would write, write nothing
tiger changed main              # calls each function started or stopped making since main
tiger changed --format markdown main   # the same report as a pull request comment
```

Every command but `tiger pattern` takes `-C dir` to run as if started in `dir`.

The analysis is deterministic static analysis and never a language model (ADR-0015). Tiger
itself calls no outside service. It loads packages through the `go` command, which fetches a
missing module the way `go build` would.

### `tiger check` and `tiger budget`

Exit codes are 0 clean, 1 findings, and 2 operational failure. A package that fails to load is
never reported as clean.

A rule either blocks or it is not a rule (ADR-0012). Blocking findings fail the run. The other
findings are advisory: every escape directive, every skipped test, and the few rules the
registry marks advisory, which the Rule Reference lists by severity. They are counted per
package against `tiger.budget.yaml`. Under budget they print nothing. A package over one of its
rows, or with counted findings for a rule it has no row for, fails the run with a `TS-D06` line
followed by the counted findings.

A synthesized loop variant is a computed fact, not a rule. It prints only under `--show-facts`,
in exact, freeze-ready pin syntax, and is what `tiger pin` freezes.

Output is deterministic and sorted by position. Tiger's own CI runs the check twice and diffs
the bytes, with and without `--show-facts`.

### `tiger changed`

`tiger changed <base>` compares the merge base of `<base>` and HEAD with the working tree and
prints, for each function, the calls it added and removed in other packages (or in functions of
its own package that call another package), then what the report could not see. It exits 0 for
any report and 2 when no report can be produced. Flags go before `<base>`.
`tiger changed --format json <base>` prints the same report as one JSON object
(`tiger.changed/v1`) that also gives every call site as a file and line, tagged `base` or
`head`, for an agent reviewer. ADR-0030 records the decision.

`tiger changed` answers whether a standard library function reaches package `syscall` from a
reach file it keeps in Go's build cache (`go env GOCACHE`), never in the project. The file is
keyed by the tiger binary, the Go version, GOOS, GOARCH, `CGO_ENABLED`, `GOEXPERIMENT`,
`GOFLAGS`, `GOFIPS140` and the CPU-level variables such as `GOAMD64`, so settings that change
the standard library never share an answer.

- When the file is missing, damaged or was removed by `go clean -cache`, tiger regenerates it
  from source, which takes a few seconds.
- Go's own cache trim deletes it after five days without a run, except under `GOCACHEPROG`,
  where Go never trims and nothing removes it.
- A CI job that already caches `GOCACHE`, as `actions/setup-go` does, keeps it with no extra
  step. A cache restored from a run that used a different tiger build misses, because the key
  names the binary, and tiger regenerates the file.
- When the base and head `go.mod` files select different Go toolchains (a `toolchain` line or
  `GOTOOLCHAIN`), each tree reads the reach file for its own toolchain, and the first run with
  such a pair makes two.
- No CI runner covers Windows, so whether tiger can replace the reach file while another run
  holds it open is untested there.
- When tiger cannot write the reach file but Go can still build, for example because something
  else sits at its path, the report still prints and stderr says the file could not be saved.
- With `GOCACHE=off` or a read-only cache Go cannot build the standard library either, and
  `tiger changed` exits 2.

## Posting the report on a pull request

`--format markdown` prints the report as a pull request comment. Its first line is
`<!-- tiger-changed -->`, the marker the workflow below uses to find its one comment and update
it on every push, so a pull request never collects a stack of them. The comment stays under
GitHub's 65,536-character limit. Past 60,000 characters it drops each new package's call list,
then cuts the main list after its last whole function and says how many calls it left out.
Tiger never talks to GitHub. The workflow posts.

```yaml
name: tiger-changed
on: pull_request

permissions:
  contents: read
  pull-requests: write

# One run per pull request at a time. Without this, two quick pushes can both find
# no comment and post two, or an older run can finish last and overwrite the newer report.
concurrency:
  group: tiger-changed-${{ github.event.number }}
  cancel-in-progress: true

jobs:
  changed:
    runs-on: ubuntu-latest
    steps:
      # Check out the pull request's own head commit, not the default merge commit
      # (refs/pull/N/merge). The comment's heading names this commit, and the merge base must
      # be the fork point that "Files changed" compares against. fetch-depth 0
      # keeps the history tiger needs to find it; without it tiger exits 2.
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go install github.com/kapetan-io/tiger/cmd/tiger@latest   # or a pinned commit, see Install
      - name: post the report as one sticky comment
        env:
          GH_TOKEN: ${{ github.token }}
          BASE_REF: ${{ github.base_ref }}
        # The report goes outside the checkout: a new file in it would make tiger
        # label the head "+ uncommitted changes".
        run: |
          tiger changed --format markdown "origin/$BASE_REF" > "$RUNNER_TEMP/changed.md"
          tiger changed "origin/$BASE_REF" >> "$GITHUB_STEP_SUMMARY"
          id=$(gh api repos/${{ github.repository }}/issues/${{ github.event.number }}/comments --paginate \
            --jq '.[] | select(.body | startswith("<!-- tiger-changed -->")) | .id' | head -1)
          if [ -n "$id" ]; then
            gh api -X PATCH repos/${{ github.repository }}/issues/comments/$id -F body=@"$RUNNER_TEMP/changed.md"
          else
            gh api repos/${{ github.repository }}/issues/${{ github.event.number }}/comments -F body=@"$RUNNER_TEMP/changed.md"
          fi
```

Every run posts, an empty report included, because a comment that says what tiger compared and
what it cannot see is the evidence that it ran on this head. If tiger exits 2 the step fails
before anything is posted. Pull requests from forks get a read-only token, so this job cannot
comment on them.

## The committed files

### `tiger.yaml`

`tiger.yaml` holds the facts a repository tells the analyzers. Every analyzer entry is a value,
a reason, and an optional package scope, reviewed like code. An entry widens what an analyzer
detects or corrects a dictionary. No entry names a function, file, or site to exempt
(ADR-0003, ADR-0005), with one exception, `comments.allow`, below. When the file exists, setting the same analyzer flag on the command line
is an error, because the committed file is the reviewed source.

```yaml
version: 1
participle:
  allow:
    - value: binding
      reason: RoleBinding is an association record, not an action
      packages: [internal/auth/...]
ioinloop:
  packages:
    - value: github.com/jackc/pgx/v5
      reason: every call is a network round trip
comments:
  allow:
    - value: internal/legacy
      reason: imported from v1; reasons move into tests under PLAT-212
```

The `comments.allow` entries are directories rather than package scopes. TS-L14 blocks prose
comments in production code, and an entry exempts a directory's whole subtree (`.` is the whole
module). It is the one entry that names a place to exempt, a brownfield adoption path that
review of `tiger.yaml` guards (ADR-0031).

A `nolint` block with `forbid: true` and a reason turns on the ban on `//nolint`, so
`tiger check` reports every one in a checked package. The
[Rule Reference's TS-L09 entry](Tiger%20Rule%20Reference.md) shows the block and the finding.

### `tiger.budget.yaml`

`tiger.budget.yaml` holds what the repository admits it owes, one number per package per
counted rule. The [Explainer's section on the budget](Tiger%20Explainer.md#the-budget-and-the-ratchet)
says how `tiger budget --write` lowers it and why nothing raises it (ADR-0011). The first run
after adoption fails on every package carrying an escape or a skipped test until
`tiger budget --write` records the counts once.

```yaml
internal/pin:
  TS-L09: 1
internal/store:
  TS-D07: 3
```

### `tiger.layers.yaml`

`tiger.layers.yaml` is optional and declares the import graph (TS-X03). The
[Explainer's section on the layer file](Tiger%20Explainer.md#the-layer-file-says-what-each-package-may-import)
shows its format and what fails. `config/tiger.layers.yaml` in this repository is a template to
copy next to `go.mod`.

## Directives

The [Explainer's section on directives](Tiger%20Explainer.md#directives) lists every verb.
There is deliberately no `//tiger:bounded` and no dismissal directive. Tiger has no
package-level declarations (ADR-0024), and the `hot`, `wire`, and `owner` verbs do not exist
(ADR-0028).

## Running under golangci-lint

The plugin in `plugin/` registers every analyzer as the custom linter `tiger`.
`golangci-lint custom`, run next to a `.custom-gcl.yml`, builds a golangci-lint binary with the
plugin inside. Tiger's own `.custom-gcl.yml` is the example. It names the module
`github.com/kapetan-io/tiger` and the import `github.com/kapetan-io/tiger/plugin`, and its
`path: .` line points at a tiger checkout, so an adopter's copy replaces that line with a
`version:` naming the tiger commit to build. The
project's `.golangci.yml` enables `tiger` under `linters.enable` with
`linters.settings.custom.tiger.type: module`, plus these settings and one habit:

- `issues.uniq-by-line: false` is required. golangci-lint keeps one issue per line by default,
  so a second finding on the same line, such as a `TS-N07` and a `TS-N08` on one signature, is
  dropped silently. Tiger's findings are all actionable, and none may be hidden by another.
- `linters.settings.custom.tiger.settings.config` is optional and names the directory holding
  `tiger.yaml` and `tiger.layers.yaml`. By default the plugin reads them from the working
  directory, which is the module root under golangci-lint. A load error fails the plugin's
  construction with the same one-line message `tiger check` prints.
- Run `golangci-lint cache clean` after rebuilding the plugin. The issue cache key is the sorted
  analyzer names, salted with the golangci-lint version string, the `linters.settings` block
  and `go.mod`. It does not include the plugin binary, so a rebuilt plugin keeps serving the
  previous build's results until the cache is cleared. It does not include `tiger.layers.yaml`
  either, which is why the plugin adds an analyzer named `layerfile_<digest>`, so an edited
  layer file changes the key. `tiger.yaml` has the same gap and nothing closes it yet, so clear
  the cache after editing it.

The plugin differs from `tiger check` where a check needs the whole module at once, and where
golangci-lint's own behavior applies:

- **`//nolint` works.** golangci-lint honors `//nolint:tiger` and its `exclusions` for tiger's
  findings as for any linter's, so CI still gates on `tiger check`, where `//nolint` changes
  nothing.
- **Budgets are CLI-only.** The plugin API has no end-of-run hook to aggregate counts across
  packages and no warning tier, so it reports every counted finding (escape directive, skipped
  test) as an issue. A repository that carries any budgeted debt needs `tiger check` for the
  verdict.
- **TS-A07 and TS-A09 are CLI-only.** Their findings are decided in a finish step the tiger
  driver runs once after every package has been visited (ADR-0010). Under the plugin their
  per-package halves still run and export facts, and those codes are absent.
- **TS-X03's directory checks are CLI-only.** An entry in `tiger.layers.yaml` that matches no
  package directory, and a package with no entry that the run did not load, need the module's
  directories, which the plugin never sees. Its import and missing-entry findings for the
  packages it loads still fire.
- **TS-T12's sweep of test files is CLI-only.** `tiger check` also reads each checked package's
  `_test.go` files from disk and reports internal test files the analyzer did not. The plugin
  does not.

Findings from a rule that a pattern backs end with ``— see `tiger pattern <name>` ``, or
``— see `tiger pattern --rule <code>` `` when more than one pattern backs the rule, under the
plugin as under `tiger check`. The pattern text lives in the `tiger` binary, so a project that
runs only the plugin still installs `tiger`, built from the same commit as the plugin, for its
agents to run `tiger pattern`. Tiger's CI publishes both binaries together as the
`tiger-and-plugin` artifact.
