# Tiger Go

Tiger is a static analyzer for Go, built for code that AI agents write. It holds
that code to the restrictions NASA's
[Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) and TigerBeetle's
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md)
put on flight and database software, so the result is testable and free of
whole classes of production bugs. Every loop has a bound, every goroutine has an
owner, and every clock, random source, and IO call is passed in, so any function
can be tested in isolation. A run either passes or fails. `tiger check` has no
warnings and honors no suppression comment, so code that fails the check is
rewritten until it passes. The analysis is deterministic static analysis and never a language
model, with no network. **The specification lives in the source and a machine
checks the code against it.**

## Documentation

- [Tiger Explainer](docs/Tiger%20Explainer.md) — start here: what tiger is
  for and why, the concepts (injected nondeterminism, invariants, patterns,
  `tiger changed`, budgets, directives), and the real bugs its rules caught in
  trials.
- [Tiger Rule Reference](docs/Tiger%20Rule%20Reference.md) — one entry per
  enforced rule: what it requires and why, a firing example, the compliant
  rewrite, severity, and directive interactions. A doc meta-test in
  `internal/rules` keeps it in lockstep with the registry.
- [Tiger Specification](docs/Tiger%20Specification.md) — the normative
  document (Parts I–V, IDs `TS-*`) that governs everything in this
  repository.

## The tiger tool

```
tiger check ./...               # run the registered custom-rule analyzers
tiger check --show-facts ./...  # also print computed facts in pin syntax
tiger budget ./...              # report packages over their budget rows
tiger budget --write ./...      # lower tiger.budget.yaml to the current counts
tiger golangci                  # audit .golangci.yml against the auto-rule baseline
tiger golangci --init           # generate the baseline config for a new project
tiger golangci --print          # write the baseline to stdout for hand merging
tiger changed main              # calls each function started or stopped making since main
tiger changed --format markdown main   # the same report as a pull request comment
```

`tiger changed <base>` compares the merge base of `<base>` and HEAD with the
working tree and prints, for each function, the calls it added and removed in
other packages (or in functions of its own package that call another package), then what the report could not see. It exits 0 for any report
and 2 when no report can be produced. Flags go before `<base>`. ADR-0030
records the rule.

### Posting the report on a pull request

`--format markdown` prints the report as a pull request comment. Its first line is
`<!-- tiger-changed -->`, the marker the workflow below uses to find its one comment and update it
on every push, so a pull request never collects a stack of them. The comment stays under GitHub's
65,536-character limit: past 60,000 characters it drops each new package's call list, then cuts the
main list after its last whole function and says how many calls it left out. Tiger never talks to
GitHub; the workflow posts.

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
      - run: go install github.com/kapetan-io/tiger/cmd/tiger@<version>   # a released tag, as in "Running under golangci-lint" below
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

Every run posts, an empty report included, because a comment that says what tiger compared and what
it cannot see is the evidence that it ran on this head. If tiger exits 2 the step fails before
anything is posted. Pull requests from forks get a read-only token, so this job cannot comment on them.

Exit codes: 0 clean, 1 findings, 2 operational failure (a package that fails
to load is never reported as clean). A rule either blocks or it is not a rule
(ADR-0012): blocking findings fail the run; the only other findings are the
standing advisory notices the specification names — every escape directive
and every skipped test — which are counted per package against
`tiger.budget.yaml`: under budget they print nothing, and a package over its
row (or with no row) fails the run with a `TS-D06` line followed by the
counted findings. A synthesized loop variant is a computed fact, not a rule:
it prints only under `--show-facts`, in exact, freeze-ready pin syntax, and is
what `tiger pin` freezes. (Today `--show-facts` also prints effect sets and
frames; ADR-0026 removes them.) Output is
deterministic and position-sorted; CI runs the check twice and diffs the
bytes, with and without the facts channel.

### The committed files

`tiger.yaml` holds the facts a repository tells the analyzers: every entry
is a value, a reason, and an optional package scope, reviewed like code. An
entry widens what an analyzer detects or corrects a dictionary; no entry
names a function, file, or site to exempt (ADR-0003/0005). When the file
exists, setting the same analyzer flag on the command line is an error —
the committed file is the reviewed source.

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
```

`tiger.budget.yaml` holds what the repository admits it owes: one number per
package per counted rule. `tiger check` compares counts to numbers;
`tiger budget --write` creates missing rows and lowers existing ones to the
current counts, deleting rows that reach 0, and never raises a number
(ADR-0011). A raise is a hand edit in a pull request. The first run after
adoption fails on every package carrying an escape or a skipped test until
`tiger budget --write` records the counts once.

```yaml
internal/cli:
  TS-L09: 1
internal/store:
  TS-D07: 3
```

`tiger.layers.yaml`, optional, declares the import graph (TS-X03): every
package gets one entry listing what it may import beyond the pure standard
library, and `tiger check` fails an import its entry doesn't list, a package
with no entry, and an entry that matches no package directory.
`config/tiger.layers.yaml` is the template; without the file, imports are not
checked.

```yaml
"./internal/store":
  - database
  - ./internal/types
  - github.com/jackc/pgx/v5
"./internal/types": []
"./service":
  - ./internal/store
```

Two engines enforce the rules:

- **Auto rules** are enforced by off-the-shelf golangci-lint linters.
  `tiger golangci` audits that a project's config actually enforces the
  baseline; `--init` generates it from the rule registry.
- **Custom rules** are enforced by the analyzers in `internal/analyzers/`.
  The wave-1 set covers every rule computable from a single package's AST
  and types (`nogoto`, `paniccheck`, `boundedloop`, `compoundcond`,
  `nogoroutine`, `selectctx`, `chandecl`, `errignore`, `returnarity`,
  `directives`, `skipcheck`, `tablename`, `testdoc`, `derivation`,
  `limitrelate`, `sametypeparams`, `participle`, `deferdistance`); the SSA
  wave adds the analyzers that check what code does rather than how it is
  written (`norecursion`, `maporder`, `poolzero`, `effects`, `frames`,
  `variant`, `contracts`), with effect sets and frames crossing package
  boundaries through `go/analysis` facts; the cross-package wave adds the
  rules whose evidence spans the module (`invariantrefs`,
  `invariantnegative`). ENG-191 removes `effects` and `frames` (ADR-0026); the
  specification's analyzer table shows what is built and what is decided. Analyzers are driver-agnostic
  `go/analysis` passes; severity and exit codes live in the driver, per
  ADR-0002. TS-A07 and TS-A09 are whole-program rules: their finding is decided in a finish step the tiger driver
  runs once after every package has been visited (ADR-0010). Only
  `tiger check` reports them; under the golangci-lint plugin their
  per-package halves still run and export facts, and those codes are simply
  absent.

Directives share the `//tiger:<verb>` namespace, owned by the grammar package
(`internal/directive`): an unknown verb is a blocking error, never a silently
meaningless comment. Tiger admits two escape hatches.
`//tiger:batched <reason>` waives only the rule against IO inside a loop
(TS-M10, ADR-0014). `//tiger:openenum <reason>` claims a type's values are open
to growth, which tiger cannot check. Each is counted against its package's
budget on every run, so the debt it carries is a number in
`tiger.budget.yaml`'s diff (ADR-0003, ADR-0011). There is deliberately no
`//tiger:bounded` and no dismissal directive. Tiger has no package-level
declarations (ADR-0024), and the `hot`, `wire`, and `owner` verbs do not exist
(ADR-0028).

## What is here

| Path | What it is |
| --- | --- |
| `cmd/tiger/` | The CLI: a thin `main` over a testable run function. |
| `internal/pattern/` | Reads the embedded patterns: the card sections, the Go files, lookup by name and by rule. |
| `internal/rules/` | The rule registry — the single source of the rule set. The binary's analyzer set, the finish functions, the corpus meta-tests, severity, the computed-facts table, and the `tiger golangci` audit are all derived from it. |
| `internal/analyzers/` | The 31 analyzers, one package per analyzer, each with its corpus (failure-mode fires, compliant rewrite silent, known misses marked): an `analysistest` corpus for a per-package rule, a small module under `testdata/module/` run through the tiger driver for a whole-program rule. The shared internals live under `internal/analyzers/internal/`: `words` (identifier tokenization), `ssalib` (shared `go/ssa` call-graph walks used by `norecursion`, plus the effect lattice plumbing and curated stdlib effects table, which ADR-0026 removes with `effects` and `frames`), `assertpkg` (recognizes tiger's assert package by import path), and `invariants` (invariant const and assert-call collection). |
| `internal/analyzers/layers/` | TS-X03 (and TS-D01): judges every non-test import against the package's `tiger.layers.yaml` entry, exactly, and reports a package with no entry. Its corpus installs its own layer file. |
| `internal/layerfile/` | `tiger.layers.yaml`: loading and validation, the process-wide install the drivers do and the analyzer queries, the exact import judgment, the coverage comparison `tiger check` uses to report entries that match no package directory, and the wording of every TS-X03 message (ADR-0029). |
| `internal/config/` | `tiger.yaml`, plus the module facts the committed files key on: the module path from `go.mod` and the module's package directories. |
| `internal/directive/` | The `//tiger:` grammar: closed verb vocabulary, per-verb pin argument grammars (variant expressions and contract predicates; the effect lattice and frame lists go with ADR-0026), canonical printing, and the round-trip contract `Parse(Format(d)) == d`. |
| `plugin/` + `.custom-gcl.yml` | The golangci-lint module plugin: the same analyzers under `golangci-lint run`, minus the finish step — TS-A07, TS-A09 and TS-X03's directory checks (stale layer entries, packages the run did not load) are `tiger check`-only. See "Running under golangci-lint" below. |
| `assert/` | The always-on assertion package, including `Invariant`/`Violates` generic over `~string`. Zero dependencies. Import it as `github.com/kapetan-io/tiger/assert`; tiger recognizes it by that import path only. |
| `patterns/` | The patterns collection (ADR-0025): for each gap a rule cannot check, runnable code showing the right shape beside the broken one that passes `tiger check`, a `PATTERN.md` card, and a reviewer checklist. The binary embeds it: `tiger pattern <name>`. `patterns/ledger/` is the invariant vocabulary pattern: an `inv` package declaring IDs (TS-A07), a symmetric encode/decode pair asserting them, and a violation test per invariant (TS-A09). |
| `config/golangci.yml` | The Stage 0 golangci-lint v2 template with rule-ID comments. `tiger golangci --init` generates the machine-audited baseline from the registry. |
| `config/tiger.layers.yaml` | The layer file template (TS-X03): copy it next to `go.mod` and give every package an entry listing what it may import beyond the pure standard library. |

This repository dogfoods itself: CI runs Stage 0 golangci-lint plus
`tiger check ./...` over the tree against the committed `tiger.budget.yaml`,
green.

## Running under golangci-lint

The plugin registers every analyzer as the custom linter `tiger`; build it
with `golangci-lint custom` next to `.custom-gcl.yml`. Two settings are
required in the project's `.golangci.yml`, and one habit:

- `issues.uniq-by-line: false`. golangci-lint keeps one issue per line by
  default, so a second finding on the same line — a `TS-N07` and a `TS-N08`
  on one signature — is dropped silently. Tiger's findings are all
  actionable; none may be hidden by another.
- `linters.settings.custom.tiger.settings.config`, optionally, to name the
  directory holding `tiger.yaml` and `tiger.layers.yaml`. By default the
  plugin reads them from the working directory, which is the module root
  under golangci-lint. A load error fails the plugin's construction with the
  same one-line message `tiger check` prints.
- Run `golangci-lint cache clean` after rebuilding the plugin. The issue
  cache key is the sorted analyzer names, salted with the golangci-lint
  version string, the `linters.settings` block and `go.mod`. It does not
  include the plugin binary, so a rebuilt plugin keeps serving the previous
  build's results until the cache is cleared. It does not include
  `tiger.layers.yaml` either, which is why the plugin adds an analyzer named
  `layerfile_<digest>`: an edited layer file changes the key. `tiger.yaml`
  has the same gap and nothing closes it yet, so clear the cache after
  editing it.

The plugin diverges from the CLI in one way: budgets are CLI-only. The
plugin API has no end-of-run hook to aggregate counts across packages and
no warning tier, so it reports every counted finding (escape directive,
skipped test) as an issue. A repository that carries any budgeted debt needs
`tiger check` for the verdict.

TS-X03's directory checks are CLI-only for a similar reason: an entry in
`tiger.layers.yaml` that matches no package directory, and a package with no
entry that the run did not load, need the module's directories, which the
plugin never sees. Its import and missing-entry findings for the packages it
loads still fire.

Findings from a rule that a pattern backs end with ``— see `tiger pattern <name>` ``, under the plugin as under
`tiger check`. The pattern text lives in the `tiger` binary, so a project that runs only the plugin still
installs `tiger` (`go install github.com/kapetan-io/tiger/cmd/tiger@<version>   # a released tag, as in "Running under golangci-lint" below`, the same version as the
plugin) for its agents to run `tiger pattern`. CI publishes both binaries together as the
`tiger-and-plugin` artifact.

## The acceptance contract

There are no effort estimates. Implementation is AI-driven; the binding
constraint is functional acceptance, and every analyzer defines its own before
it merges:

- **An `analysistest` corpus per rule it enforces.** The rule's failure-mode
  example (from the spec's "Why") must fire the diagnostic; the compliant
  rewrite must stay silent. The corpus is the rule's functional requirement,
  executable — and a registry meta-test proves no registered analyzer lacks
  one.
- **Report output is byte-identical to pin syntax.** The grammar package owns
  both directions; every fact `--show-facts` prints is `directive.Format`
  output the grammar parses back to the same structure, so freezing a fact
  into a pin is a paste.
- **No silent scope cuts.** If an analyzer covers less than its spec rows
  claim, the corpus contains a case documenting the known miss, marked as
  such.

An analyzer without its corpus does not merge, no matter how plausible its
implementation looks.
