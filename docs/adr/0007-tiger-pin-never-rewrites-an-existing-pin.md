# 7. tiger pin never rewrites an existing pin

Date: 2026-08-26

## Status

Accepted

## Context

A pin freezes an analyzer-computed fact — an effect set, a frame, a variant — into a blocking
contract. The analyzer compares pin against code in both directions on every run, so a pin that no
longer matches the code is a blocking finding, not a stale comment. That disagreement is the entire
value of a pin: it is what turns an observation into a promise a change has to honor.

`tiger pin` writes those directives mechanically. The obvious convenience is a flag that rewrites a
pin already present so it matches the current computed fact, for the case where a deliberate
refactor legitimately changed what a function does.

The tension is that the dialect's primary consumer is an AI coding agent, and an agent offered a
cheaper path to a green build than conforming will take it. Such a flag is that path. Every blocking
pin violation — including the ones that represent a real regression the pin was written to catch —
clears with one command that touches no logic. The dialect already refuses free-text dismissal
directives on exactly this reasoning: a mechanism is judged by the cheapest thing it makes possible,
not by its intended use. A command that edits pins is that same mechanism wearing a different
interface.

The cost of refusing is real. A refactor that genuinely widens a function's effect set now needs the
pin edited by hand.

## Decision

We will give `tiger pin` an insert-only writer: it appends directives and has no operation that
deletes or replaces one. A target whose existing pin disagrees with the computed fact is refused —
`pin` prints the pinned line and the computed line and exits with the findings code, writing nothing
for that target while continuing with the rest.

The two exits from a disagreement stay what the specification defines: change the code, or edit the
pin by hand so the new promise appears in the diff and a reviewer rules on it.

## Consequences

- The refusal is structural rather than a policy check. No caller, present or future, can express a
  replacement, so the guarantee cannot erode through a later flag added in good faith.
- The cheapest parseable path out of a pin violation stays the code fix or a human-authored,
  reviewable pin edit.
- A pin edit is always a deliberate act visible in a pull request diff, which is what keeps a
  reviewer in front of every widened promise.
- Deliberate refactors that change a function's computed facts cost a hand edit per pin. On a
  refactor touching many pinned functions, that is tedious.
- If hand editing becomes the dominant workflow, the answer is a mode that emits a reviewable diff
  for a human to apply, not an in-place rewrite.
