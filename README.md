# Tiger Go

A verified Go dialect adapted from TigerBeetle's
[TIGER_STYLE](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md),
restricted so that **the specification lives in the source and a machine checks
the code against it**. Declarations are reviewed by humans; code is checked by
the `tiger` analyzer, deterministically, with no LLM and no network.

The specification (145 rules, Parts I–V, IDs `TS-*`) currently lives in the
design vault and governs everything in this repository.

## The tiger tool

```
tiger check ./...               # run the registered custom-rule analyzers
tiger check --show-facts ./...  # also print computed facts in pin syntax
tiger golangci                  # audit .golangci.yml against the auto-rule baseline
tiger golangci --init           # generate the baseline config for a new project
```

Exit codes: 0 clean, 1 findings, 2 operational failure (a package that fails
to load is never reported as clean). A rule either blocks or it is not a rule
(ADR-0012): blocking findings fail the run; the only other findings are the
standing advisory notices the specification names — every escape directive
and every skipped test, on every run — which print and count but never fail.
Computed facts — effect sets, frames, synthesized loop variants — are not
rules: they print only under `--show-facts`, in exact, freeze-ready pin
syntax, and are what `tiger pin` freezes. Output is deterministic and
position-sorted; CI runs the check twice and diffs the bytes, with and
without the facts channel.

Two engines enforce the dialect:

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
`//tiger:batched <reason>`, and it surfaces as a standing advisory finding on
every run — escapes are never silent (ADR-0003). There is deliberately no
`//tiger:bounded` and no dismissal directive. A package states its
restrictions with `//tiger:restrict closed-dispatch, no-reflect,
imports(internal/domain/...)` in its doc comment; absence of a declaration is
never a finding, a declaration its own imports or dispatch contradict is.

## What is here

| Path | What it is |
| --- | --- |
| `cmd/tiger/` | The CLI: a thin `main` over a testable run function. |
| `internal/rules/` | The rule registry — the single source of the dialect. The binary's analyzer set, the finish functions, the corpus meta-tests, severity, the computed-facts table, and the `tiger golangci` audit are all derived from it. |
| `internal/analyzers/` | The 33 analyzers, one package per analyzer, each with its corpus (failure-mode fires, compliant rewrite silent, known misses marked): an `analysistest` corpus for a per-package rule, a small module under `testdata/module/` run through the tiger driver for a whole-program rule. The shared internals live under `internal/analyzers/internal/`: `words` (identifier tokenization), `ssalib` (the effect lattice plumbing over `go/ssa`, including the curated stdlib effects table), `restrict` (the package restriction declaration), and `invariants` (invariant const and assert-call collection). |
| `internal/directive/` | The `//tiger:` grammar: closed verb vocabulary, per-verb pin argument grammars (the effect lattice, frame lists, variant expressions, contract predicates), canonical printing, and the round-trip contract `Parse(Format(d)) == d`. |
| `plugin/` + `.custom-gcl.yml` | The golangci-lint module plugin: the same analyzers under `golangci-lint run`, minus the finish step — TS-A07, TS-A09, and TS-X01 are `tiger check`-only. |
| `assert/` | The always-on assertion package, including `Invariant`/`Violates` generic over `~string`. Zero dependencies. Copy it to `internal/assert` in your project. |
| `examples/ledger/` | The invariant vocabulary pattern: an `inv` package declaring IDs (TS-A07), a symmetric encode/decode pair asserting them (TS-A08), and a violation test per invariant (TS-A09). |
| `config/golangci.yml` | The Stage 0 golangci-lint v2 template with rule-ID comments. `tiger golangci --init` generates the machine-audited baseline from the registry. |

This repository dogfoods itself: CI runs Stage 0 golangci-lint plus
`tiger check ./...` over the tree, green.

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
