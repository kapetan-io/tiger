# 28. Tiger has no hot, wire, or owner markers

Date: 2026-10-12

## Status

Accepted

## Context

The specification defines three intent directives that mark code for stricter rules:
`//tiger:hot` marks a hot-path function, `//tiger:wire` marks a type whose layout is a serialized
format, and `//tiger:owner` marks a type as the single owner of its state. Seven rules apply only to
marked code:

- TS-F03, TS-M02, TS-M08 and TS-M11 hold hot paths to zero allocation and an effect set of at most
  `{mutate}`, and cross-check the marks against the profiler.
- TS-M06 field-aligns wire and hot structs with explicit padding.
- TS-K06 makes the field order of wire types declared and stable.
- TS-C10 gives an owner type no exported fields, no `sync` primitives, and no copying.

TS-S17 also used `//tiger:wire` to find types whose size assertions it checks, and TS-M07 named hot
paths.

None of the seven rules is built. The parser accepts the three verbs, but no analyzer reads them,
which is why ENG-191 (decision.md, the canceled-tickets table, ENG-178 item 4) disposed of the
verbs. The hot-path rules also leaned on the effects analyzer that ADR-0026 removes. Keeping the
rules as unbuilt goals would keep a set of features in the specification that are not part of what
tiger is.

## Decision

The `hot`, `wire` and `owner` verbs are removed from the directive vocabulary, and the rules that
exist only for marked code are retired: TS-M02, TS-M06, TS-M08, TS-M11, TS-K06 and TS-C10. TS-F03
is already retired by ADR-0026.

Two rules stay without the markers:

- TS-M07 is what tiger ships today: the `perfsprint` linter, applied everywhere. Its "in hot paths"
  scope and its `alloc` effect are dropped.
- TS-S17, asserting relationships between constants at compile time, is enforced by review.

## Consequences

- The directive vocabulary is the pins `variant`, `requires` and `ensures`, the intent declaration
  `openenum`, and the escape `batched`.
- An unknown verb is a blocking TS-L09 finding, so code that still carries `//tiger:hot`, `wire` or
  `owner` fails until the comment is removed.
- TS-C11 (no exported method returns a pointer, slice, or map aliasing an internal field) stays a
  rule for every type, so part of what TS-C10 protected remains. It is not built yet.
- Allocation on hot paths and wire-format layout are left to benchmarks and review.
