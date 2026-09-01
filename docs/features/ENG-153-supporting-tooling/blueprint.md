# Supporting Tooling Blueprint: budget ratchet, config file, golangci print

Linear: ENG-153. First PR of the parked "supporting tooling" collection. Auto-fix, editor/LSP
integration, dashboards, JSON output, and interface-pinned signature exemptions are split into
follow-up tickets (see Scope).

## Objective

Three gaps the two real-codebase trials (ENG-148 querator, ENG-159 git-server) and the advisory
audit (ENG-162) left open:

1. **Nothing stops advisory debt from growing.** Escape-directive uses (TS-L09) and skipped tests
   (TS-D07) print on every run and never fail one. ADR-0003 admitted the escape hatch on the promise
   of "its accounting"; ADR-0006 called the two standing advisories "the input a per-package ratchet
   is built to count." The ratchet is TS-D06 in the specification and has no implementation.
2. **Per-repo vocabulary lives in single-string flags.** `-participle.allow` and
   `-ioinloop.packages` cannot carry a reason, cannot be scoped to a package, and are not
   reviewed like code. ENG-161 removed three naming rules partly because a global flag cannot express
   a token that is domain vocabulary in one service and lazy in another. The naming-dictionary rules
   stay dead until a committed, scoped, reasoned config file exists.
3. **`tiger golangci` names every missing entry and offers no merged config.** The audit listed 46
   unenforced auto rules on querator; the adopter hand-edited 46 entries. `--init` refuses when a
   config exists.

Outcome: a repo commits `tiger.yaml` (vocabulary, hand-edited) and `tiger.budget.yaml` (advisory
budgets, tool-written, only ever lowered by the tool); `tiger check` fails when a package exceeds
its budget; `tiger golangci --print` emits the baseline for hand merging.

## Mental Model

- **Config file** (`tiger.yaml`): facts the repo tells the analyzers about the world. Every entry
  is a value, a reason, and an optional package scope. A reviewer reads it like code. An entry
  widens what an analyzer detects or corrects a dictionary; no entry names a function, file, or
  site to exempt.
- **Budget file** (`tiger.budget.yaml`): what the repo admits it owes. One number per package per
  counted rule. `tiger check` compares counts to numbers. `tiger budget --write` lowers numbers to
  match reality. A number goes up only by a hand edit in a pull request. This file, not check
  output, is where a reader sees which deviations a package carries.
- **Ratchet**: the rule that budgets only decrease. Tiger enforces the "count ≤ budget" half;
  review enforces the "never raised without a commit that says so" half, exactly as the
  specification's TS-D06 text describes.
- **Baseline print**: `tiger golangci --print` writes the same YAML `--init` would create, to stdout,
  whether or not a config exists.

## Core Design Principles

1. **Tiger never raises a budget.** The write command creates missing rows and lowers existing
   ones; it has no code path that increases an existing number. Same reasoning as ADR-0007: a
   mechanism is judged by the cheapest thing it lets an AI agent do.
2. **A config key passes an admission test: it may widen what an analyzer detects or correct a
   dictionary; it may never name a function, file, or site to exempt.** `ioinloop.packages`
   widens; `participle.allow` corrects a dictionary (a reviewer is the truth check on "this
   token is a noun", as with an escape reason). `nogoroutine`'s `-supervisors` flag fails the
   test: a function listed there has every `go` statement inside it exempted, which is the per-site
   deviation directive in config shape that ADR-0003/0005 gate. This PR removes that flag; the
   compliant path for a goroutine is `errgroup.Group.Go` or another library that owns the join,
   until `nogoroutine` computes the supervisor shape (follow-up ticket).
3. **Analyzers stay driver-agnostic (ADR-0002).** The config loader is shared; each driver (CLI,
   golangci plugin) loads the file and installs it before running. Analyzers never read files.
   `analysistest` installs nothing and sees empty config.
4. **Every printed line obeys ADR-0009.** Ratchet and config errors are one line, ≤240 characters
   of body, exactly one `TS-` code, no banned vocabulary.
5. **Two files, two authors.** Human text and tool-written numbers never share a file, so the tool
   never has to round-trip comments.
6. **Binary verdicts, no warnings.** `tiger check` prints blocking findings and a verdict, nothing
   else. A counted finding (escape directive, skipped test) under budget prints nothing; on
   overrun it prints as a blocking line under the `TS-D06` line. Slack is visible only in the
   budget file's diff. The `[advisory]` marker, the advisory count in the trailer, and the
   `reported` severity tier (removed by ENG-152) are gone; nothing here targets them.

## Correctness Constraints

### State invariants

- **I1. A budget row is a non-negative integer keyed by (module-relative package path, rule code
  of an advisory-severity registry entry).** Violation: unknown package pattern, unknown or
  non-advisory rule code, negative or non-integer value. Detected at load; `tiger check` and
  `tiger budget` exit 2 naming the row.
- **I2. After `tiger budget --write`, every written row equals the current count for that package
  and rule, and no written row is greater than the row it replaced.** Violation is impossible by
  construction: the writer computes `min(existing, count)` for existing rows, `count` for new rows,
  and deletes rows whose count is 0.
- **I3. Every config entry has a non-empty reason.** Violation detected at load; exit 2 naming the
  entry.
- **I4. The budget file is byte-deterministic for a given set of rows.** Keys sorted, fixed
  indentation, no comments. Two runs over the same tree produce identical files.

### Behavioral constraints

- **B1. `tiger check` never mutates `tiger.budget.yaml`.** Only `tiger budget --write` writes it.
- **B2. A budget write is atomic.** Temp file in the same directory, then rename. A crash never
  leaves a half-written file.
- **B3. A package with advisory findings and no budget row fails the run** (implicit budget 0).
  Omission cannot dodge the ratchet.
- **B4. When `tiger.yaml` exists, an explicitly set analyzer flag on the command line is an error
  (exit 2).** Committed config is the reviewed source; a flag cannot silently override it.
- **B5. `tiger check` on a package subset checks only the analyzed packages.** Budgets for packages
  outside the run are neither checked nor reported.
- **B6. The ratchet counts analyzer findings only, never its own lines.** TS-D06 lines are
  driver-produced and excluded from counting.
- **B7. Under budget, counted findings print nothing.** There is no output between "clean" and
  "failed".

Concurrency: two pull requests that both lower budgets conflict on merge and are resolved by hand;
that conflict is the review gate the specification asks for. Reversibility: lowering is reversible
only by a hand edit; that is the point. Partial failure: B2 covers the write; a failed load fails
the whole run before any finding prints.

## Acceptance Criteria

Ratchet:

1. `tiger check ./...` on a tree with 4 skipped tests in `internal/cli` and a row `internal/cli:
   {TS-D07: 2}` exits 1 and prints one `TS-D06` line positioned at the row followed by the four
   `TS-D07` findings as blocking lines.
2. Same tree, row `TS-D07: 4`: exits 0 and prints nothing.
3. Same tree, row `TS-D07: 6`: exits 0 and prints nothing.
4. Same tree, no row for `internal/cli` (file present or absent): exits 1 with a `TS-D06` line
   positioned at the file path with no line number.
5. `tiger budget --write` on the tree from (3) rewrites the row to `4`, exits 0, and a second run
   produces a byte-identical file.
6. `tiger budget --write` on the tree from (1) leaves the row at `2`, exits 1, and still lowers
   every other over-budgeted row in the same run.
7. `tiger budget --write` deletes a row whose count is now 0 and removes a package block with no
   rows left.
8. A row naming a blocking rule code (`TS-S02`) or an unregistered code exits 2 from both
   `tiger check` and `tiger budget`.
9. `tiger check ./internal/cli` does not report budgets for `internal/driver`.
10. `.github/report.awk` loses its advisory table and `[advisory]` branch; the trailer regex
    matches `tiger: N blocking`; the golden fixture gains a TS-D06 overrun and the counted lines
    under it.
11. Every TS-D06 message template passes the ADR-0009 mechanical checks (one line, ≤240, one code,
    banned words) in a fixture test.

Config file:

12. A `tiger.yaml` entry `participle.allow: [{value: binding, reason: "...", packages:
    [internal/auth/...]}]` silences TS-N14 on `RoleBinding` in `internal/auth` and still fires on
    the same identifier in `internal/api`.
13. An entry with no `packages` applies to the whole module.
14. An entry with an empty or missing reason exits 2 naming the analyzer, key, and value.
15. An unknown analyzer name or unknown key for a known analyzer exits 2 naming it.
16. `tiger check -participle.allow=x` with `tiger.yaml` present exits 2 naming the flag.
17. `tiger check -participle.allow=x` without `tiger.yaml` behaves as today.
18. The golangci plugin, run on a module with `tiger.yaml`, applies the same entries (plugin-smoke job
    extended with a scoped entry).
19. `analysistest` corpora run unchanged with no config installed.
19a. `tiger check -nogoroutine.supervisors=x` exits 2 as an unknown flag; the TS-C02 message ends
    with the `errgroup.Group.Go` edit and no longer mentions a flag; the nogoroutine corpus and
    every `// want` regex move with it.

golangci:

20. `tiger golangci --print` on a module with an existing `.golangci.yml` writes the generated
    baseline to stdout, writes no file, exits 0.
21. `--print` output is byte-identical to what `--init` writes on a fresh module with the same
    module path.
22. `--print` without a `go.mod` exits 2 with the same message `--init` gives.

Docs:

23. README plugin section prescribes `issues.uniq-by-line: false` and `golangci-lint cache clean`
    after plugin rebuilds, with the reason for each.
24. The specification's TS-D06 "Enforce" line names `tiger check` and `tiger budget`, and the
    Diagnostics section lists TS-D06's position convention.
25. Tiger's own tree commits a `tiger.budget.yaml` (currently `internal/cli: {TS-L09: 1}`) and the
    dogfood job stays green.

## Scope

### In scope (this PR)

- `internal/config`: loader, schema validation, package-scope matching, install hook for drivers.
- `internal/budget`: budget file load/compare/write.
- `tiger check`: config install, flag-conflict check, ratchet comparison, TS-D06 lines.
- `tiger budget` subcommand: report (default) and `--write`.
- `tiger golangci --print`.
- Plugin: load `tiger.yaml` from the working directory (path overridable by a plugin setting).
- Migration of `participle.allow` and `ioinloop.packages` to read config entries in addition to
  flags.
- Removal of `nogoroutine`'s `-supervisors` flag and its allowlist; the analyzer's message names
  `errgroup.Group.Go` as the edit.
- Registry: a driver-enforced rule kind for TS-D06 (blocking) with a per-rule `counted` noun
  phrase on advisory entries.
- `tiger check` output: drop the `[advisory]` marker and the advisory trailer count; update
  `.github/report.awk`, its golden fixture, and every CLI test that asserted them.
- README plugin docs; specification edits; tiger's own `tiger.budget.yaml`.

### Out of scope / follow-up tickets

Each becomes its own Linear ticket under the Tiger project:

- **Auto-fix** for rules with one compliant shape (ADR-0008 already fixes the mechanism: a second
  consumer of the placement writer).
- **Editor/LSP integration.**
- **Dashboards / trend reporting** of advisory counts over time.
- **JSON output** for `tiger check` (the prefix contract is documented; a positive structured
  format is separate work).
- **Interface-pinned signature exemptions** (`porcupine.Model.Step` under TS-E06). This is the
  per-site deviation form in config shape; it needs an ADR superseding the ADR-0003/0005 gate, now
  that the ratchet exists.
- **Reviving naming-dictionary rules** (`noabbrev`, `restatement`, TS-N12/13/15) on top of the
  config file.
- **Computed supervisor shape for `nogoroutine`**: recognize `wg.Add` before the `go` and
  `wg.Wait` in the same function or the owner's `Close` (10 of querator's 14 TS-C02 findings), so
  a repo-local supervisor needs no declaration.
- **Complexity, assertion-density, and allocation metrics** for the ratchet: they need analyzers
  that compute them; the ratchet accepts any advisory rule code, so they join without ratchet
  changes.
- Git-aware raise detection. Review is the gate.
- **Silencing-channel audit follow-ups** (ENG-177 recognize behavior not names; ENG-178 close
  uncounted silencing channels: generated-file header, `//nolint:tiger` under the plugin,
  `package assert` by name, dead intent verbs; ENG-179 `openenum` and TS-E02's comment become
  counted escapes). ENG-178 and ENG-179 need this PR's ratchet to count.

## Dependencies and Constraints

- ADR-0002 (driver-agnostic analyzers), ADR-0003/0005 (no waivers in config), ADR-0007 (never
  raise), ADR-0009 (message invariants). No ADR is departed from. ADR-0011 records the
  lowering-only budget policy; the in-flight ENG-152 branch claims 0010 on its own branch, so
  re-check `docs/adr/` on `origin/main` at rebase time and renumber if the two collide.
- `gopkg.in/yaml.v3` is already a dependency.
- The golangci-lint plugin API has no end-of-run hook, so the ratchet runs under the CLI only.
  With no ratchet and no warning tier, the plugin reports every counted finding as an issue; a
  repo that carries any budgeted debt needs the CLI. Documented as the plugin divergence.
- ENG-152 removes the `reported` severity tier; this work targets `blocking` and `advisory` only,
  and redefines `advisory` as "counted against the budget file" (see Functional).

---

## Functional

### `tiger.yaml`

Location: the directory `tiger check` runs in (`-C`), which must contain `go.mod`. No upward
walk, matching `.golangci.yml` lookup. Absent file means empty config and today's behavior.

Shape: `version: 1`, then one map per analyzer name, then one list per flag name on that analyzer.
Every list element is `{value, reason, packages}`; `packages` is optional and holds module-relative
Go package patterns (`internal/auth`, `internal/auth/...`, `.`). Example:

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

Validation at load, all exit 2 with one line each: unknown `version`; analyzer name not in the
registry; key not a flag on that analyzer; empty `value`; empty `reason`; malformed pattern. The
loader knows nothing about what a value means; the analyzer does.

Flag conflict: after parsing, if `tiger.yaml` loaded and any `<analyzer>.<flag>` flag was explicitly
set, exit 2: `tiger check: -participle.allow is set on the command line but tiger.yaml is the
reviewed source — move the value into tiger.yaml`.

Delivery to analyzers: the shared config package holds the installed config; an analyzer asks for
the values of its own key that apply to `pass.Pkg.Path()`. The package relativizes the import path
against the module path it loaded from `go.mod`. Analyzers union those values with their flag
(flags stay for repos without a file and for `analysistest`). This is process-global state, the
same shape as analyzer flags today (`internal/cli/flags_test.go` documents the reset discipline).

Plugin: reads `tiger.yaml` from the working directory at `BuildAnalyzers` time; golangci runs from
the module root. A `config` key in the plugin's settings overrides the path. A load error fails
plugin construction with the same one-line message.

### `tiger.budget.yaml`

Location: same directory as `tiger.yaml`. Shape: a map from module-relative package path to a map
from rule code to integer. Sorted keys, two-space indent, no comments, trailing newline:

```yaml
internal/cli:
  TS-L09: 1
internal/store:
  TS-D07: 3
  TS-L09: 2
```

Rule codes are the user-facing code (`TS-L09`), not the registry category (`TS-L09-escape`); the
registry maps advisory categories to codes. Only categories registered at advisory severity may
appear. A code that is registered but not advisory (demoted, promoted) is an exit-2 error so a
stale row is noticed the moment severity changes.

### `tiger check` ratchet

Advisory severity means one thing after this change: the finding is counted per package against
the budget file. It never prints on its own. After findings are partitioned by severity, count
advisory findings per (package, rule code) over the analyzed packages. For each (package, code):

- count > budget (including missing row, budget 0): one blocking `TS-D06` line, then every
  counted finding for that package and code printed as an ordinary blocking line (`TS-D07: ...`,
  no marker), so the reader sees which sites to fix.
- count ≤ budget: nothing.

Position of the `TS-D06` line: the budget file path plus the row's line number when the row
exists; the path alone (line 0) when it does not. The trailer becomes `tiger: N blocking`; the
`[advisory]` marker and the advisory count leave the CLI with this change.

Message templates (`<counted>` is the registry's noun phrase for the rule, e.g. "skipped tests",
"escape directives"; singular form when the count is 1):

- Overrun: `TS-D06: internal/cli has 4 skipped tests but tiger.budget.yaml allows 2 — fix 2 of
  them, or raise the number in a reviewed edit`
- Missing row: `TS-D06: internal/cli has 4 skipped tests and tiger.budget.yaml has no row for it —
  fix them, or run tiger budget --write to record the current count`

The ratchet lines never enter the ratchet's own counts (B6). Exit code: overrun contributes to
`ExitFindings` like any blocking finding.

### `tiger budget`

`tiger budget [-C dir] [packages...]` runs the same analysis as `tiger check` over the packages
(default `./...`), prints the overrun lines only (no other analyzer findings), and exits 1 on
overrun, 0 otherwise. Within budget it prints nothing.

`tiger budget --write` additionally rewrites `tiger.budget.yaml`: for every analyzed package and
advisory code, new row = `min(existing, count)` if a row exists, `count` if not; rows at 0 are
deleted; packages with no rows are deleted; packages outside the run are untouched. Overrun rows
are left as they were and still reported; the command exits 1 when any overrun remains, 0
otherwise. Write is temp-and-rename. Prints nothing beyond the overrun lines; the file diff is
the report of what was lowered.

### `tiger golangci --print`

Writes `Generate(modulePath)` (including its head comment) to stdout; never touches the
filesystem beyond reading `go.mod`. Exit 0, or 2 without `go.mod`. `--init` and `--print` together
are an error (exit 2).

### Plugin documentation

README plugin section gains two prescriptions with their reasons: `issues.uniq-by-line: false`
(golangci keeps one issue per line by default, dropping same-line findings) and `golangci-lint
cache clean` after rebuilding the plugin (the analysis cache keys on source, not on the plugin
binary). Plus the ratchet divergence: budgets are CLI-only.

## Architecture

- `internal/config` — owns the `tiger.yaml` schema, validation, module relativization, package
  pattern matching, and the installed-config holder analyzers query. Contract: given (package
  import path, analyzer name, key) returns the applicable values; returns nothing when no config is
  installed. Error category: a single load error type carrying analyzer/key/value for the one-line
  message. Both drivers call it; analyzers depend only on the query.
- `internal/budget` — owns the budget file: load with line numbers per row (for positions),
  validation against the registry, comparison producing the three finding kinds, and the
  lowering-only writer. Contract: the writer's inputs are the loaded file and the current counts;
  its output never contains a number greater than the input's for the same key. The driver
  supplies counts; the package never runs analysis.
- `internal/rules` — registry gains a driver-enforced entry kind (no analyzer, no corpus) for
  `TS-D06` at blocking severity, and a `counted` noun phrase on every advisory custom rule,
  checked by the coherence meta-test (an advisory rule without one cannot register). `ByCategory`
  resolves the driver category like any other.
- `internal/cli` — `check.go` installs config, rejects conflicting flags, runs the comparison,
  prints ratchet lines; new `budget.go` for the subcommand; `golangci.go` gains `--print`; `usage`
  updated.
- `plugin` — installs config in `newPlugin`; error surfaces as the plugin construction error.
- `participle` and `ioinloop` query config for their key and union with the flag. `nogoroutine`
  loses its flag. No other analyzer changes.

Domain-boundary opacity: analyzers depend on the config query's contract (values applicable to
this package), never on which driver installed it or whether a file existed. The budget package
depends on counts, never on the driver or the analyzers. Neither driver branches on the other.

## Data Design

Both files are pinned above. The budget writer emits through a sorted `yaml.Node` walk like
`internal/golangci/generate.go`, so I4 holds by construction.

### Invariant preservation

- I1: enforced at load in `internal/budget`; nothing else constructs rows.
- I2: enforced by the writer's arithmetic; there is no API that sets a row to an arbitrary value.
  Application logic, covered by acceptance criteria 5–7 and a property-style test (random existing
  budgets and counts, assert the output row ≤ input row for every key).
- I3: enforced at load in `internal/config`.
- I4: structural, from the sorted encoder; determinism test double-runs the writer.

### Illegal state analysis

Illegal states made unrepresentable: rows keyed by non-advisory codes (rejected before any
comparison), negative budgets (rejected at load), tool-raised budgets (no code path). Left to
application logic: the flag-conflict check (B4), which is a CLI-level guard covered by criterion
16.

## Security

Config values widen what analyzers detect or correct a dictionary; they never exempt a site and
never change what tiger executes. Pattern matching is a Go package-pattern match on import paths, not a filesystem glob;
no path escapes the module. Budget write targets a fixed filename in the run directory.

## PII

None. Both files hold identifiers, package paths, and free-text reasons authored by the repo.

## Scale

Counting is a map increment over findings already in memory. The budget file grows by one row per
(package, advisory rule) with debt; a 500-package monorepo with two advisory rules is at most
1,000 rows.

## Testing

Testing follows the `surface-testing` skill. The surface is `cli.Run(args, Streams)`; no test
calls `internal/config` or `internal/budget` directly except the property test on the writer's
arithmetic, which is justified by I2 being the load-bearing invariant.

Key surfaces:

- integration (`internal/cli`, package `cli_test`, fixtures under `testdata/fixtures/budget/*` and
  `testdata/fixtures/config/*`, each a tiny module): criteria 1–17, 20–22. Write-path tests copy the
  fixture into `t.TempDir()` (existing `copyFixture`). Determinism tests double-run `--write` and
  `--print`.
- meta-test (`internal/rules`): registry coherence for the driver-rule kind and the `counted`
  field; ADR-0009 style checks over TS-D06 templates rendered through a fixture run (criterion 11).
- report fixture (`.github/testdata/report.txt` + golden): drop the advisory section, add a
  TS-D06 overrun with its counted lines (criterion 10).
- plugin-smoke CI job: fixture module with a scoped `tiger.yaml` entry; assert the finding is absent
  in the scoped package and present outside it (criterion 18).
- fakes needed: none. No external dependency, no clock, no async behavior.

## Limitations & Future Work

- The ratchet runs under the CLI only; a golangci-only adopter has no budget enforcement.
- `tiger budget --write` can recreate a row that was deleted by hand, which reads in the diff as a
  raise from absent to N. Review catches it; the tool cannot distinguish first adoption from a
  deleted row without git.
- File-level scoping (as opposed to package-level) is not offered; no current rule needs it.
- Implicit budget 0 means an adopting repo's first `tiger check` after upgrade fails on every
  package with a skipped test or escape until `tiger budget --write` runs once. The missing-row
  message names that command.

## Open Questions

None. A per-package table from `tiger budget` was considered and rejected: check and budget
output is a verdict, not a report; the budget file is the report.
