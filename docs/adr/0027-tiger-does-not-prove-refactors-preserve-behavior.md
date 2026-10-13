# 27. Tiger does not prove refactors preserve behavior

Date: 2026-10-12

## Status

Accepted

## Context

The specification's Part I builds a chain toward one payoff. With no globals, every function's
reachable heap is its parameters and receivers. With that, each function's frame (the state it may
write) is computable. With frames and effect sets, a tool can prove an extract-function,
inline-function, or rename leaves behavior unchanged, by checking that the region's effect and
frame sets do not change. TS-R01 is that proof, enforced by a `refactor` transform tool that writes a
`Refactor-Proof:` commit trailer CI re-checks. A proven refactor would merge without review, and the
specification's headline measure, the fraction of merged lines no human read, rests on it.

ADR-0026 removes effect and frame pins, the effect table, and the effects and frames analyzers, and
replaces them with the `tiger changed` report. No decision names a new way to compute frames, so
TS-R01 has no mechanism left. Nothing of it is built: there is no `refactor` tool and no trailer.

Keeping TS-R01 as a goal with an undecided method would leave the specification promising a feature
no part of tiger delivers. The explainer was rewritten on the same principle: it describes what
tiger is meant to be, not what it once hoped to be.

## Decision

Tiger does not prove that a refactor preserves behavior. TS-R01 and the `refactor` tool are retired.

The specification's argument ends at what tiger still delivers. With no package-level mutable state,
the functions that could have written a value are few, and a reviewer can name them. `tiger changed`
shows which struct started calling something new. A refactoring diff gets the same review as any
other diff.

## Consequences

- Part I's chain, Chain 6's last step, and the "lines no human read" headline are reworded or
  removed. The specification no longer claims a class of diff that needs no review.
- TS-R02 (public surface diff) and TS-R03 (generated and hand-written code face the same gate) stay,
  without the effect sets and package restrictions they used to list.
- The no-globals rule, TS-C11, and unique identifiers stay. They were justified by the refactoring
  proof, and they still shrink the set of code that could have written a value.
