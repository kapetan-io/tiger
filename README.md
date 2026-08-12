# Tiger Go

A verified Go dialect adapted from TigerBeetle's
[TIGER_STYLE](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md),
restricted so that **the specification lives in the source and a machine checks
the code against it**. Declarations are reviewed by humans; code is checked by
the `tiger` analyzer, deterministically, with no LLM and no network.

The specification (145 rules, Parts I–V) currently lives in the design vault
and governs everything in this repository.

## What is here now

| Path | What it is |
| --- | --- |
| `assert/` | The always-on assertion package, including `Invariant`/`Violates` generic over `~string`. Zero dependencies. Copy it to `internal/assert` in your project. |
| `examples/ledger/` | The invariant vocabulary pattern: an `inv` package declaring IDs (TS-A07), a symmetric encode/decode pair asserting them (TS-A08), and a violation test per invariant (TS-A09). Living proof that `assert` couples to `inv` by shape, not by import. |
| `config/golangci.yml` | The Stage 0 golangci-lint v2 configuration. Every linter maps to a rule ID. Copy it into your project and replace `myrepo` with your module path. |

## What gets built next

The computed-only wave first — analyzers that need no declarations:

1. `nogoroutine`, `paniccheck`, `boundedloop`, and the other small AST passes
2. Chain 1 ownership analyzers (`ownership`, `escapecheck`, `chandecl`, …)
3. Chain 2 surfaces (`surfaces`, `adapters`), `closedworld`, `canonical`, `surfacediff`
4. The `effects` engine, inference-first (per the NilAway blueprint); pin
   enforcement lands only after reporting works

All analyzers are `golang.org/x/tools/go/analysis` passes, shipped as one
binary (`tiger check ./...`) and as a golangci-lint module plugin.

## The acceptance contract

There are no effort estimates. Implementation is AI-driven; the binding
constraint is functional acceptance, and every analyzer defines its own before
it merges:

- **An `analysistest` corpus per rule it enforces.** The rule's failure-mode
  example (from the spec's "Why") must fire the diagnostic; the compliant
  rewrite must stay silent. The corpus is the rule's functional requirement,
  executable.
- **Report output is byte-identical to pin syntax.** A reported effect set is
  the exact `//tiger:...` line a developer would paste. A test asserts the
  bytes.
- **No silent scope cuts.** If an analyzer covers less than its spec rows
  claim (heuristic, advisory-only, partial predicate language), the corpus
  contains a case documenting the known miss, marked as such.

An analyzer without its corpus does not merge, no matter how plausible its
implementation looks.
