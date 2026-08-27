# 9. Diagnostics are written for a reader new to tiger

Date: 2026-08-27

## Status

Accepted

## Context

Tiger's diagnostics are the first thing an adopting team reads, hundreds of lines of them on the
first run. The wave-1 blueprint set AI agents as the primary consumer and required every finding
to name its compliant form. Both held, and the messages that resulted were correct and
self-correcting; they were also written in tiger's own vocabulary:

```
TS-F02: computed effects io(net) are not declared by this pin — introduced by a call to helper.Ping at core.go:12:20 — remove the call or update the pin to //tiger:effects io(net)
```

A reader who has never seen a `//tiger:` directive meets three terms of art (computed effect,
pin, declared by) before the one thing they need: this function does network IO that its comment
does not mention. Messages also pointed at sibling rules ("or use the ctx.Done() event-loop shape
(TS-S03)"), which breaks any tool that keys a line on its rule code, and several ran past 300
characters of rationale that belongs in the specification.

Nothing enforced any of this. Each analyzer wrote its own message, the only mechanical check was
that the message led with its rule ID, and wording drift was invisible until a human read the
output on a real tree.

## Decision

Every message tiger prints is written for a newcomer human: someone with no tiger context must be
able to say, after one read, what the code did and the exact edit to make. This supersedes the
wave-1 "AI agents are the primary consumer" line as the wording standard. The self-correcting
requirement stays. An agent that can act on a newcomer's message can act on any message; the
reverse is false, and the newcomer decides whether tiger gets adopted.

A diagnostic is one line with two halves. The prefix `path:line:col: TS-XXX:` is a parsing
contract and never changes. The body is prose in three parts, separated by ` — `: what the code
does, what tiger expected, and the edit. The first two may merge when one clause carries both;
the edit is never omitted from a blocking finding. The `--show-facts` channel prints what tiger
found, says there is nothing to fix, and ends with the directive that would freeze the fact.

The body uses no internal vocabulary. The banned list starts with pin, pinned, computed, fact,
facts, effect set, frame condition, lattice, closed set, back edge, dominating, synthesized,
ranking, allowlist. Directives are spelled literally (`//tiger:effects io(net)`), never described.
The edit is an edit: a comment to add, a call to remove, a name to change. Rationale stays out
unless the edit would look arbitrary without it. A body names exactly one rule code, in the
prefix. A body is one line, at most 240 characters on corpus output, 160 by preference.

The specification's "Diagnostics" section is the operative checklist. The corpus meta-test in
`internal/rules` and a fixture test in `internal/golangci` enforce the mechanical half on every
`go test`: no second rule code, no newline, the length cap, the banned list, and `//tiger:`
wherever a directive verb appears. A blind reader given the message text alone checks the rest
before a message ships.

## Consequences

- A message that fails the checklist does not merge, and the failure names the analyzer, the
  message, and the invariant. Wording drift stops being a review-time catch.
- The banned list lives in two places, the test and the specification, and they change
  together. Adding a term to one without the other is the drift the test exists to catch, so the
  specification section is the test's human-readable twin, not an independent document.
- Directive verbs (`effects`, `frame`, `variant`, `requires`, `ensures`, `batched`) cannot be
  banned as vocabulary without false positives on literal directives. The mechanical check
  requires `//tiger:` in any body that uses one; the blind reader is the guard against their
  misuse as prose.
- Runtime messages interpolate real identifiers and can exceed 240 characters (norecursion
  joins every name in a cycle). The cap binds templates as exercised by the corpus.
- A rule code written as a directive verb (`//tiger:TS-N07`, the unshipped deviation form) is
  reported as `//tiger:<rule code>` rather than echoed, so the body keeps one rule code.
- Every `// want` regex in the corpus, every exact-string CLI test, and the CI report fixtures
  moved with the wording, in the same change. A future rewording pays the same cost, which is
  the point: the corpus is each rule's executable specification, and the message is part of it.
- ENG-154's reference documentation quotes these messages and adds the reasoning the messages
  no longer carry. The why lives there and in the specification, not on the finding line.
