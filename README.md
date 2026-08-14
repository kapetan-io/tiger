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
tiger check ./...        # run the wave-1 custom-rule analyzers
tiger golangci           # audit .golangci.yml against the auto-rule baseline
tiger golangci --init    # generate the baseline config for a new project
```

Exit codes: 0 clean, 1 findings, 2 operational failure (a package that fails
to load is never reported as clean). Blocking findings fail the run; advisory
findings — every escape directive in the tree, on every run — print and count
but never fail. Output is deterministic and position-sorted; CI runs the check
twice and diffs the bytes.

Two engines enforce the dialect:

- **Auto rules** are enforced by off-the-shelf golangci-lint linters.
  `tiger golangci` audits that a project's config actually enforces the
  baseline; `--init` generates it from the rule registry.
- **Custom rules** are enforced by the wave-1 analyzers in
  `internal/analyzers/`: every rule computable from a single package's AST
  and types (`nogoto`, `paniccheck`, `boundedloop`, `compoundcond`,
  `nogoroutine`, `selectctx`, `chandecl`, `errignore`, `returnarity`,
  `directives`, `skipcheck`, `tablename`, `testdoc`, `derivation`,
  `limitrelate`, `sametypeparams`, `namedeny`, `namepairs`, `participle`,
  `deferdistance`). Analyzers are driver-agnostic `go/analysis` passes;
  severity and exit codes live in the driver, per ADR-0002.

Directives share the `//tiger:<verb>` namespace, owned by the grammar package
(`internal/directive`): an unknown verb is a blocking error, never a silently
meaningless comment. Wave 1 admits exactly one escape hatch,
`//tiger:batched <reason>`, and it surfaces as a standing advisory finding on
every run — escapes are never silent (ADR-0003). There is deliberately no
`//tiger:bounded` and no dismissal directive.

## What is here

| Path | What it is |
| --- | --- |
| `cmd/tiger/` | The CLI: a thin `main` over a testable run function. |
| `internal/rules/` | The rule registry — the single source of the dialect. The binary's analyzer set, the corpus meta-tests, severity, and the `tiger golangci` audit are all derived from it. |
| `internal/analyzers/` | The 20 wave-1 analyzers, one package per analyzer, each with its `analysistest` corpus (failure-mode fires, compliant rewrite silent, known misses marked). |
| `internal/directive/` | The `//tiger:` grammar: closed verb vocabulary, canonical printing, and the round-trip contract `Parse(Format(d)) == d`. |
| `plugin/` + `.custom-gcl.yml` | The golangci-lint module plugin: the same analyzers under `golangci-lint run`. |
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
  both directions and its round-trip property test is the wave-1 form of that
  contract; pin-printing analyzers arrive with the effects wave.
- **No silent scope cuts.** If an analyzer covers less than its spec rows
  claim, the corpus contains a case documenting the known miss, marked as
  such.

An analyzer without its corpus does not merge, no matter how plausible its
implementation looks.
