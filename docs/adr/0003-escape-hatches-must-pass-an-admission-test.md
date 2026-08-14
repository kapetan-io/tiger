# 3. Escape hatches must pass an admission test

Date: 2026-08-12

## Status

Accepted

## Context

Tiger's primary consumer is an AI coding agent, and an agent offered a cheaper path to a green
build than conforming will take it: a one-line directive with a plausible generated reason costs
less than a fix. A directive's reason can only ever be shape-checked (verb known, reason
present), never truth-checked — a tool cannot tell a real termination argument from a fabricated
one, so a free-text "trust me" directive is a suppression regardless of what it is called. The
Tiger Go Specification originally defined three escape-hatch directives (`//tiger:bounded`,
`//tiger:batched`, and a generic `//tiger:<rule-id>` deviation form), while also warning that
forced directives are how a dialect degrades and defining a false positive on a blocking rule as
a bug in the analyzer, not a condition the code must absorb. Wave-1 analyzers are exact by
construction: when one fires, the banned shape is really present, so there is no tool-error case
for an escape to serve. The tension is adopter convenience against consumer incentives.

## Decision

We will admit a per-site escape directive only where the restricted dialect eliminates something
reality requires — where no rewrite inside the dialect can comply. Inconvenience never
qualifies, and the test binds every future escape proposal: name the eliminated capability the
escape restores, and ship it with its accounting.

- `//tiger:bounded` fails the test and does not exist in the tool: every terminating loop can
  state an explicit iteration cap and assert on exhaustion, and genuinely-forever loops have the
  event-loop shape (select on `ctx.Done()`).
- A generic dismissal directive fails the test and does not exist; an unknown `//tiger:` verb is
  a blocking error.
- `//tiger:batched` passes the test — an external system that only accepts per-item IO is a
  constraint of the world no rewrite can remove — and is wave 1's only escape.
- Every admitted escape carries a mandatory reason, is truth-reviewed by humans, and surfaces as
  a standing advisory finding on every run, so unverified claims never leave the report.
- The generic deviation form may ship only together with the heuristic rules that create genuine
  false-positive pressure and the per-package ratchet that counts it — mechanism and control in
  the same release.

## Consequences

- An AI agent cannot trade a fix for a comment: the cheapest parseable path to green is the
  compliant rewrite the finding names.
- A blocking false positive halts adopters until the analyzer is fixed; the interim fallback is
  disabling the whole analyzer in driver configuration, which is deliberately loud. This raises
  the quality bar every shipped analyzer must meet.
- Escape claims accumulate on the advisory report instead of disappearing into the tree,
  substituting for ratchet tooling until it exists.
- Under the golangci-lint driver, golangci's own `//nolint` still applies to any linter it runs;
  that door is outside tiger's control.
