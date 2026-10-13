# 24. Packages declare no restrictions

Date: 2026-10-12

## Status

Accepted

Supersedes the TS-P02 paragraph of [ADR-0012](0012-a-rule-either-blocks-or-does-not-exist.md),
since TS-P02 is removed.

## Context

A package could declare restrictions in its doc comment with `//tiger:restrict`, using three
axes: `closed-dispatch` (every method call resolves to one concrete type, never through an
interface), `no-reflect`, and `imports(...)`. TS-P01 checked the package's own imports against the
declaration. TS-P02 checked that every dependency declared at least as much. TS-P03 required a
directive to weaken a declaration. TS-K03 flagged interface calls in a closed-dispatch package, and
TS-F01 required an effect pin on every exported function of one. A package that declared nothing
got no finding.

ENG-191 call 11 started as a question about whether the explainer should restore the directive,
and ended by asking whether any axis earns its place
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, call 11).

- `no-reflect` repeats TS-S12, which already bans `reflect` everywhere through golangci-lint.
- `imports(...)` repeats TS-D01's default and the TS-X03 layer file.
- `closed-dispatch` is not from TigerStyle or Power of Ten (the nearest ancestor is Power of Ten's
  ban on function pointers), and no codebase declares it. Declaring it on querator's `internal`
  package produces 68 blocking findings. 26 are calls through `Partition`, the interface
  querator's four storage backends implement, which is the architecture. Twelve are calls on
  `context.Context`, including `ctx.Done()`, which TS-S03 and TS-C05 require. A probe that waits
  the way TS-C05 requires gets two TS-K03 findings on `ctx.Done()` and `ctx.Err()`, and the named
  fix is impossible, because no concrete type exists for a `context.Context` you are handed.
- TS-P02 fires on every third-party import, because third-party code declares nothing.

## Decision

`//tiger:restrict` is removed, with TS-P01, P02, P03 and K03, the `restrictions` and `closedworld`
analyzers, and the rule that requires pins on every exported function of a closed-dispatch package.
Tiger has no package-level declarations.

Reflection stays banned everywhere by TS-S12. Import restrictions stay with TS-D01 and the TS-X03
layer file.

## Consequences

- No adopter meets findings like the 68 above, and tiger's cancellation rules no longer contradict
  another tiger rule.
- A dependent can no longer check a claim that a dependency avoids reflection or interface
  dispatch. No codebase made such a claim.
- The TS-X03 layer file this decision relies on for `imports(...)` is not built. TS-X03 exists only
  in the specification, and depguard cannot fail a package that no rule covers. Until ENG-328
  builds it, layering inside a module is enforced only by the hand-written depguard rules in
  `config/golangci.yml`.
- Effect pins lose their closed-dispatch requirement here. ADR-0026 removes effect and frame pins
  entirely.
- The specification reconciliation (ENG-191 follow-up 7) removes the directive and the four rules
  from the specification.
