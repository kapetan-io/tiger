# Tiger Source Layout

This page maps tiger's repository for someone changing tiger itself: what each package owns,
and how the two engines that enforce the rules divide the work. The
[User Guide](Tiger%20User%20Guide.md) covers running tiger, and the
[Specification](Tiger%20Specification.md) is the normative document every package answers to.

## Two engines

- **Auto rules** are enforced by off-the-shelf golangci-lint linters. `tiger golangci` audits
  that a project's config actually enforces the baseline, and `--init` generates it from the
  rule registry.
- **Custom rules** are enforced by the analyzers in `internal/analyzers/`. They were built in
  waves (build milestones). Wave 1 covers the rules computable from a single
  package's AST and types. A later wave, built on `go/ssa`'s static single assignment form of
  the code, adds the analyzers that check what code does rather than how it is written, such as
  `norecursion` and `variant`.
  The cross-package wave adds the rules whose evidence spans the module (`invariantrefs`,
  `invariantnegative`). The specification's analyzer table lists every analyzer and its status:
  built, built with changes decided, or not built.

Analyzers are driver-agnostic `go/analysis` passes. Severity and exit codes belong to whatever
runs them, the tiger CLI or the golangci-lint plugin, never to an analyzer (ADR-0002). In the
CLI, `internal/check/` applies them. TS-A07 and TS-A09 are whole-program rules, decided in a finish step the tiger
driver runs once after every package has been visited (ADR-0010).

## Packages

| Path | What it is |
| --- | --- |
| `cmd/tiger/` | The CLI: a thin `main` over a testable run function. |
| `internal/cli/` | The `tiger` command's name lookup and command listing, and the `Run` function `main` wraps. |
| `internal/command/` | A subcommand's contract with the shell: its output streams, the exit codes, and the shape of `-h` output. |
| `internal/rules/` | The rule registry, the single source of the rule set. The binary's analyzer set, the finish functions, the corpus meta-tests, severity and the computed-facts table are all derived from it. |
| `internal/rules/auto/` | The auto rules, each bound to its golangci-lint linter and baseline settings. The `tiger golangci` audit and the `--init` generator are derived from them. |
| `internal/golangci/` | `tiger golangci`: audits a project's golangci-lint config against the auto-rule baseline, and generates that baseline with `--init` or `--print`. |
| `internal/analyzers/` | The 28 analyzers, one package per analyzer, each with its corpus (failure-mode fires, compliant rewrite silent, known misses marked): an `analysistest` corpus for a per-package rule, a small module under `testdata/module/` run through the tiger driver for a whole-program rule. |
| `internal/analyzers/internal/` | What several analyzers share. `ssalib` holds the `go/ssa` call-graph walks `norecursion` uses. `assertpkg` recognizes tiger's assert package by import path. `invariants` collects invariant consts and assert calls. `attach` resolves which directive is attached to a node. The loop rules share `condition` (the terms of a condition), `counter` (a loop's counter terms), `intrange` (an integer type's range), `mutation` (what can change a variable inside a declaration) and `cancel` (a loop that runs until its context is cancelled). The goroutine and wait rules share `ownership` (whether package code can reach a close, send or `Wait`) and `stopcase` (a select's stop case). |
| `internal/analyzers/layers/` | TS-X03 (and TS-D01): judges every non-test import against the package's `tiger.layers.yaml` entry, exactly, and reports a package with no entry. Its corpus installs its own layer file. |
| `internal/driver/` | Loads Go packages, runs the registered analyzers over them in a stable order, propagates facts across packages, runs the finish step, and returns position-sorted findings. A package that fails to load is a terminal error, never a silent shrink of the rule set. |
| `internal/finish/` | The contract between a whole-program rule and the driver that runs its finish step (ADR-0010). |
| `internal/facts/` | The message format that carries a computed fact's directive from an analyzer to `--show-facts` and `tiger pin`, and the one place that extracts it back. |
| `internal/worklist/` | `NodesMax`, the limit every tiger worklist shares, so a graph walk over a cycle fails loudly instead of hanging. |
| `internal/check/` | The check run behind `tiger check` and `tiger budget`: run setup, the driver call, the severity partition, the TS-X03 and TS-T12 coverage findings, the ratchet, and output. |
| `internal/budget/` | `tiger.budget.yaml`: loading, comparing counts against rows, and the lowering-only write (ADR-0011). |
| `internal/layerfile/` | `tiger.layers.yaml`: loading and validation, the process-wide install the drivers do and the analyzer queries, the exact import judgment, the coverage comparison `tiger check` uses to report entries that match no package directory, and the wording of every TS-X03 message (ADR-0029). |
| `internal/config/` | `tiger.yaml`: loading and validation, and the process-wide install the drivers do and the analyzers query. |
| `internal/module/` | The Go module a run sits in: the module path from `go.mod`, module-relative package paths, the package directories and nested modules, and the Go files; and every module at and below a run directory, with each nested `go.mod` it could not read. |
| `internal/pin/` | `tiger pin`: freezes computed facts into pins. |
| `internal/placement/` | Placement: collecting `//tiger:` directives and the one insertion writer (ADR-0008). |
| `internal/directive/` | The `//tiger:` grammar: closed verb vocabulary, per-verb pin argument grammars (variant expressions and contract predicates), canonical printing, and the round-trip contract `Parse(Format(d)) == d`. |
| `internal/changed/` | `tiger changed`: the report of what a revision range changes, in text, markdown and JSON. |
| `internal/changedtests/` | `tiger changed-tests`: the existing test functions a revision range modifies or deletes, in text and JSON. |
| `internal/callinventory/` | For one checkout of a module, which function makes which calls into other packages, and the reach file that says which standard library functions reach `syscall`. |
| `internal/revision/` | The two sides of a `tiger changed` or `tiger changed-tests` run: the merge base of a base ref and HEAD, against the working tree. It is the only package that runs git, so `tiger check` stays free of git (ADR-0022). |
| `internal/pattern/` | Reads the embedded patterns: the card sections, the Go files, lookup by name and by rule. |
| `internal/ci/` | Tests of tiger's own CI workflow. It holds no non-test code. |
| `plugin/` + `.custom-gcl.yml` | The golangci-lint module plugin: the same analyzers under `golangci-lint run`, minus what needs the whole module. The [User Guide](Tiger%20User%20Guide.md#running-under-golangci-lint) lists what it skips and the settings it needs. |
| `assert/` | The always-on assertion package, including `Invariant`/`Violates` generic over `~string`. It is its own module (`assert/go.mod`) with no requirements, so a project that requires it inherits nothing, not even `golang.org/x/tools`. Import it as `github.com/kapetan-io/tiger/assert`. Tiger recognizes it by that import path only. See "Changing assert" below. |
| `patterns/` | The patterns collection (ADR-0025): for each gap a rule cannot check, runnable code showing the right shape beside the broken one that passes `tiger check`, a `PATTERN.md` card, and a reviewer checklist. The binary embeds it: `tiger pattern <name>`. `patterns/ledger/` is the invariant vocabulary pattern: an `inv` package declaring IDs (TS-A07), a symmetric encode/decode pair asserting them, and a violation test per invariant (TS-A09). |
| `config/golangci.yml` | The Stage 0 golangci-lint v2 template with rule-ID comments. `tiger golangci --init` generates the machine-audited baseline from the registry. |
| `config/tiger.layers.yaml` | The layer file template (TS-X03): copy it next to `go.mod` and give every package an entry listing what it may import beyond the pure standard library. |

## Tiger checks itself

CI runs `go vet ./...`, `go test ./...`, golangci-lint at Stage 0 (the auto rules with no false
positives, per the specification's adoption ladder) plus gocritic's `hugeParam` and `rangeValCopy`
(TS-M03, so Tiger passes large structs by pointer as it tells adopters to) and `tiger check ./...` over this
repository against the committed `tiger.budget.yaml`, and every pattern must pass `tiger check`
with zero findings. A doc meta-test in `internal/rules` keeps the
[Rule Reference](Tiger%20Rule%20Reference.md) in lockstep with the registry.

## Changing assert

`assert/` is a nested module, so `./...` at the root skips it, and CI repeats its vet, tests, lint
and `tiger check` inside it.

The root module requires it at a pseudo-version with no `replace`, because `go install ...@version`
refuses a module that has one. Root code and analyzer tests therefore compile against that pinned
commit, not your working copy. To ship a change under `assert/`, merge it, then at the root run
`go get github.com/kapetan-io/tiger/assert@<commit on main>` and `go mod tidy`, and commit `go.mod`
and `go.sum`. The commit must stay reachable on main, so this repository merges with merge commits.
A CI step fails when the pinned tree differs from `assert/`, except on a pull request that edits
`assert/`.

A project still on a Tiger version that holds `assert/` in its root module gets `ambiguous import`
once it requires the `assert` module. If it uses only `assert`, it drops the root module:

```
go get github.com/kapetan-io/tiger@none github.com/kapetan-io/tiger/assert@<version>
go mod tidy
```

If it also uses the root module, it upgrades `github.com/kapetan-io/tiger` first, to a version whose
root `go.mod` requires the `assert` module.
