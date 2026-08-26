# 6. A failed advisory trial removes the rule

Date: 2026-08-26

## Status

Accepted

## Context

Tiger registers judgment-adjacent rules at advisory severity: their findings print and are
counted but never fail the run, so the rule collects evidence from real codebases before anyone
argues about promoting it to blocking. The path out of advisory was defined in one direction
only — promotion once trials show every finding actionable — leaving no stated answer for a rule
whose trial goes the other way.

The same channel carries two findings that are not heuristics and never resolve themselves: the
standing notice every escape directive raises on every run, and every skipped test. Those two are
the accounting that makes an escape hatch honest and the input a per-package ratchet is built to
count.

Advisory volume is unbounded by construction. Nothing fails, so nothing forces the count down,
and each rule's output is free to its own author and expensive to every other rule's findings.

Measured across three codebases, two advisory rules produced 892 of 909 findings; the escape and
skipped-test advisories were 12 of them. On tiger's own tree the pull-request comment read
"Exit 0 — no blocking findings" followed by 324 advisory lines.

The tension is between the tuning instinct — lower a threshold, add an exemption, keep the rule —
and the readability of the channel. A rule can be individually defensible and still be wrong to
ship, because the cost of its noise is paid by other rules' findings.

## Decision

We will treat advisory registration as a trial with two exits: promotion to blocking, or removal.

A rule is removed when its output on real codebases is dominated by findings a reader would
decline to act on — including any finding whose named remedy would break the code. Threshold
tuning is admitted only when the misfires form a bounded shape that can be named and excluded;
"raise the limit until the count looks reasonable" is not tuning, and parking a rule at advisory
indefinitely is not an outcome.

Removal is end-to-end: analyzer, corpus, registry entry, and the specification's enforcement
line. No disabled-by-default analyzer and no tombstone. Where the underlying maxim is sound and
only the mechanical check failed, the specification keeps the rule and names human review as its
enforcement.

The advisory output of a conforming tree must stay short enough to read in one screen. A rule
whose admission would break that budget does not enter the trial.

## Consequences

- Escape uses and skipped tests stay visible on the report, so the accounting that substitutes
  for ratchet tooling survives contact with new heuristic rules.
- The bar for entering the trial rises. Advisory is no longer a safe parking spot for a rule that
  cannot articulate what "every finding actionable" would look like.
- Genuine signal is discarded along with the noise. A removed rule's true positives go
  unreported, trading recall for a channel people read.
- Removal is destructive in a way severity changes are not: promotion and demotion are one-line
  registry edits, while restoring a removed rule means rewriting its analyzer and corpus. Version
  control holds the code; no configuration flag brings it back.
- A rule that fires on nothing is not removed by this bar. Zero output buries nothing, so such a
  rule stays registered pending evidence either way.
- Judging a rule requires running it on real codebases and counting per diagnostic category. A
  corpus proves an analyzer does what it claims; it can never show whether the claim is worth
  printing.
