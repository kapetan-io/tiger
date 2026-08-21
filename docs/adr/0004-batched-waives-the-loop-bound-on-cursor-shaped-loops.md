# 4. Batched waives the loop bound on cursor-shaped loops

Date: 2026-08-17

## Status

Accepted

## Context

TS-S02 requires every loop to state a bound the analyzer can verify: a constant, a `len`/`cap`,
or a counter. The rule deliberately accepts no directive waiver — the `//tiger:bounded` escape
was withdrawn because every terminating loop can state an explicit cap and assert on
exhaustion.

Trials on two real codebases found one shape where that reasoning fails: store-cursor drain
loops (`for it.Valid()`, `for rows.Next()`, pagination drains) — 37 findings, the single
largest noise source in the suite. These loops are finite because the backing store is finite,
which is a fact of the world outside the code. A synthetic cap would be arbitrary: no correct
value exists that is not either a magic number or a restatement of "however big the store is."
The same world-fact argument admitted `//tiger:batched` as the dialect's only escape (an
external system that only accepts per-item IO), and cursor drains perform per-item IO by
definition.

The tension: leaving the loops flagged forces 37 pointless rewrites; recognizing the cursor
shape as bounded outright would admit an unsound heuristic into a blocking rule (an infinite
generator has the same shape); an unrestricted directive waiver hands an AI agent a one-line
suppression for any loop.

## Decision

We will let `//tiger:batched <reason>` waive TS-S02's bound requirement on a loop, but only
when the loop matches the cursor shape: the condition is a boolean method call on an
identifier, and either that call advances the cursor or the identifier receives an advancing
method call in the body or post. One directive with one reason covers both TS-M10 and the loop
bound on the same loop. The escape continues to surface as a standing advisory finding on every
run. The specification's "no directive waives it" clause on TS-S02 is amended to carry this
single exception.

## Consequences

- The shape gate keeps the directive from becoming a blanket suppression: an annotated `for {}`
  or any non-cursor loop still fires TS-S02.
- An adversarial infinite iterator that matches the shape and carries an annotation passes
  silently — the truth of the reason is human-reviewed, as with every escape. The exactness
  guarantee of blocking rules now has this one reviewed exception.
- Pagination shapes that are not method-call cursors (for example a loop-carried token) remain
  findings and must state a bound or restructure.
