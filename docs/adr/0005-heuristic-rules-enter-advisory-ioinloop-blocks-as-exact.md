# 5. Heuristic rules enter advisory; ioinloop blocks as exact

Date: 2026-08-17

## Status

Accepted

## Context

The dialect's standing policy reserves the generic per-site deviation directive
(`//tiger:<rule-id> <reason>`) and the per-package ratchet that counts it for the release that
first ships rules with genuine false-positive pressure — mechanism and control together, never
one without the other. Wave 1.5 is the first wave of tune-heavy rules, so the question of
whether that release has arrived must be answered now.

TS-M10 (no IO inside a loop) is nominally heuristic: "is this call IO" has no complete answer.
But the rule completes the dialect's only escape hatch — `//tiger:batched` is validated and
surfaced since wave 1 while nothing consumes it — and an escape hatch only has teeth against a
rule that fails the build. A false positive on a blocking rule is defined as an analyzer bug,
so a blocking TS-M10 must not guess.

The remaining new rules (declaration order, declaration-use distance) are judgment-adjacent
with no tuning evidence yet.

## Decision

We will build the TS-M10 IO classifier conservatively so the rule is exact by construction: a
call counts as IO only when its callee is defined in a package on an explicit allowlist of
unconditionally-IO packages, resolved through type information; everything unprovable is silent
and documented as a known miss. Interface-based detection is rejected because in-memory
implementers (`bytes.Buffer` satisfies `io.Writer`) would produce false positives. On that
basis TS-M10 registers as blocking. Every other new rule in the wave registers as advisory, and
is promoted to blocking by a registry severity edit once trials on at least two real codebases
show every finding actionable. The deviation directive and the ratchet remain unbuilt: no
blocking rule with false-positive pressure ships, so the trigger for releasing them together
has not fired.

## Consequences

- `//tiger:batched` gains real force: the compliant paths out of a TS-M10 finding are the
  batched rewrite or a reasoned, permanently visible escape.
- The exact-tier guarantee survives: no blocking rule in the dialect can fire on compliant
  code.
- The conservative classifier trades recall for precision — IO behind unlisted packages or
  same-package helpers escapes detection until the allowlist grows per repo.
- Advisory findings do not fail builds, so the new rules generate tuning data without needing
  an escape or a ratchet.
- Each promotion is a one-line registry change, keeping severity policy out of analyzers.
