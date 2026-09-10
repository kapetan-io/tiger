# Tiger Go

Tiger deterministically forces AI agents to write Go that is testable and free of
whole classes of production bugs. It holds the code to the restrictions NASA's
Power of Ten and TigerBeetle's
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md)
put on flight and database software: every loop has a bound, every goroutine has
an owner, and every clock, random source, and IO call is passed in, so any
function can be tested in isolation when you need to. Tiger has no warnings: the
code passes static analysis, or the agent must rewrite the code. No LLM is
involved in the analysis. **The specification lives in the source and a machine
checks the code against it.** Declarations are reviewed by humans; code is
checked by the `tiger` analyzer, with no network.

## Documentation

- [Tiger Explainer](docs/Tiger%20Explainer.md) — start here: why tiger
  exists, the real bugs it caught in trials, how it differs from a linter,
  and the concepts (verdicts, budgets, directives, pins) you'll meet in a
  day's work.
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
```

Exit codes: 0 clean, 1 findings, 2 operational failure (a package that fails
to load is never reported as clean). A rule either blocks or it is not a rule
(ADR-0012): blocking findings fail the run; the only other findings are the
standing advisory notices the specification names — every escape directive
and every skipped test — which are counted per package against
`tiger.budget.yaml`: under budget they print nothing, and a package over its
row (or with no row) fails the run with a `TS-D06` line followed by the
counted findings. Computed facts — effect sets, frames, synthesized loop
variants — are not rules: they print only under `--show-facts`, in exact,
freeze-ready pin syntax, and are what `tiger pin` freezes. Output is
deterministic and position-sorted; CI runs the check twice and diffs the
bytes, with and without the facts channel.

### The two committed files

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
  rules whose evidence spans the module (`restrictions`, `closedworld`,
  `invariantrefs`, `invariantnegative`, `singleimpl`). Analyzers are
  driver-agnostic `go/analysis` passes; severity and exit codes live in the
  driver, per ADR-0002. Three rules — TS-A07, TS-A09, TS-X01 — are
  whole-program rules: their finding is decided in a finish step the tiger
  driver runs once after every package has been visited (ADR-0010). Only
  `tiger check` reports them; under the golangci-lint plugin their
  per-package halves still run and export facts, and those three codes are
  simply absent.

Directives share the `//tiger:<verb>` namespace, owned by the grammar package
(`internal/directive`): an unknown verb is a blocking error, never a silently
meaningless comment. Wave 1 admits exactly one escape hatch,
`//tiger:batched <reason>`, and it is counted against its package's budget on
every run — escapes are never silent: the debt they carry is a number in
`tiger.budget.yaml`'s diff (ADR-0003, ADR-0011). There is deliberately no
`//tiger:bounded` and no dismissal directive. A package states its
restrictions with `//tiger:restrict closed-dispatch, no-reflect,
imports(internal/domain/...)` in its doc comment; absence of a declaration is
never a finding, a declaration its own imports or dispatch contradict is.

## What is here

| Path | What it is |
| --- | --- |
| `cmd/tiger/` | The CLI: a thin `main` over a testable run function. |
| `internal/rules/` | The rule registry — the single source of the rule set. The binary's analyzer set, the finish functions, the corpus meta-tests, severity, the computed-facts table, and the `tiger golangci` audit are all derived from it. |
| `internal/analyzers/` | The 33 analyzers, one package per analyzer, each with its corpus (failure-mode fires, compliant rewrite silent, known misses marked): an `analysistest` corpus for a per-package rule, a small module under `testdata/module/` run through the tiger driver for a whole-program rule. The shared internals live under `internal/analyzers/internal/`: `words` (identifier tokenization), `ssalib` (the effect lattice plumbing over `go/ssa`, including the curated stdlib effects table), `restrict` (the package restriction declaration), and `invariants` (invariant const and assert-call collection). |
| `internal/directive/` | The `//tiger:` grammar: closed verb vocabulary, per-verb pin argument grammars (the effect lattice, frame lists, variant expressions, contract predicates), canonical printing, and the round-trip contract `Parse(Format(d)) == d`. |
| `plugin/` + `.custom-gcl.yml` | The golangci-lint module plugin: the same analyzers under `golangci-lint run`, minus the finish step — TS-A07, TS-A09, and TS-X01 are `tiger check`-only. See "Running under golangci-lint" below. |
| `assert/` | The always-on assertion package, including `Invariant`/`Violates` generic over `~string`. Zero dependencies. Copy it to `internal/assert` in your project. |
| `examples/ledger/` | The invariant vocabulary pattern: an `inv` package declaring IDs (TS-A07), a symmetric encode/decode pair asserting them (TS-A08), and a violation test per invariant (TS-A09). |
| `config/golangci.yml` | The Stage 0 golangci-lint v2 template with rule-ID comments. `tiger golangci --init` generates the machine-audited baseline from the registry. |

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
  directory holding `tiger.yaml`. By default the plugin reads it from the
  working directory, which is the module root under golangci-lint. A load
  error fails the plugin's construction with the same one-line message
  `tiger check` prints.
- Run `golangci-lint cache clean` after rebuilding the plugin. The analysis
  cache keys on source files, not on the plugin binary, so a rebuilt plugin
  keeps serving the previous build's results until the cache is cleared.

The plugin diverges from the CLI in one way: budgets are CLI-only. The
plugin API has no end-of-run hook to aggregate counts across packages and
no warning tier, so it reports every counted finding (escape directive,
skipped test) as an issue. A repository that carries any budgeted debt needs
`tiger check` for the verdict.

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
