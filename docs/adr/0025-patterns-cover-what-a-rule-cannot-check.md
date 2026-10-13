# 25. Patterns cover what a rule cannot check

Date: 2026-10-12

## Status

Accepted

## Context

Some correct shapes can be checked by tiger only in part. The shutdown loop is the clearest case.
Tiger can check how the loop goroutine starts, that its only exit is its stop case, and that the
stop case is a real context or a channel the package closes. It cannot check that the loop runs
cleanup before it closes `done`, that `Shutdown` lets the caller's context bound only its wait, or
that every caller waiting on the loop's reply also selects on `done`. Querator's shutdown bugs
(ENG-160, ENG-196) lived in exactly those parts. An agent fixing a TS-S03 finding improvises a
shutdown loop, and querator shows what improvised shutdown loops look like.

`examples/ledger` is already a worked pattern of this kind, and the specification's "How review
divides" section already assigns some questions to reviewers. ENG-191 call 15 makes that a
deliberate part of tiger
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, call 15).

A deliberation weighed three ways to deliver a pattern: packages in the repository only, packages
plus the `tiger` binary, and packages plus a generated agent skill. Adopters don't have tiger's
source: querator does not depend on the tiger module at all, and copies `assert/` in. The binary is
the only delivery where the pattern an agent reads is guaranteed to match the rules that flagged
the code, and drift between the two is how a pattern collection goes stale.

## Decision

Tiger has two parts: blocking rules for what it can check exactly, and a collection of patterns for
the parts of a correct shape no rule can check.

A pattern exists only where a tiger rule stops short and a real bug lived in that gap: a trial
finding, a filed ticket, or a bug an experiment reproduced. A shape that is merely good Go, with no
rule it backs up and no bug behind it, does not qualify.

Every pattern meets all five requirements:

1. It is runnable code: a small Go package with the right shape next to the broken shape it
   replaces, and tests that demonstrate both. The broken shape passes `tiger check`, and its test
   shows the bug.
2. CI runs `tiger check` on it with zero findings and runs its tests under `-race`.
3. It names the rules it backs up and the part those rules can't see.
4. The messages of those rules point to it.
5. It gives reviewers a short checklist of yes-or-no questions for the parts tiger can't check.

Patterns live under `patterns/<name>/` in tiger's root module. `examples/` is renamed `patterns/`,
with `examples/ledger` and eight seed patterns from the ENG-191 experiments as the first entries.
The `tiger` binary embeds them: `tiger pattern <name>` prints a pattern's card, code and tests,
`tiger pattern --rule <code>` lists the patterns behind a rule, and `--checklist` prints only the
reviewer questions. Each rule's registry entry names its patterns, and the driver appends
`see \`tiger pattern <name>\`` to every finding from that rule. The analyzers do not change.
Releases ship the `tiger` binary alongside the golangci plugin.

The entry condition and the five requirements are recorded in the repository's `CLAUDE.md`.

## Consequences

- A pattern cannot fall out of date with the rules: a pattern that draws a finding fails CI, and a
  meta-test fails if a pointer names a pattern that does not exist or a pattern names a rule that
  does not exist. The message length cap counts the pointer.
- An agent that hits a finding reads the correct shape from the same release as the rule, offline
  and with no install beyond the binary. An adopter's agent instructions gain one line: for
  findings that name a pattern, run `tiger pattern <name>`.
- Adopters on the golangci plugin alone need the `tiger` binary to read a pattern.
- The entry condition keeps the collection from becoming a general Go style guide.
- Each seed must be cleaned up to meet the requirements before it is added; `experiments/call7`
  gets 28 findings today, most from its test helper and file layout.
- A reusable runtime package that encodes these shapes, such as a `lifecycle` package for the
  shutdown loop, was set aside. Following the pattern is the goal.
- No reviewer skill is built now. If one is needed, it is generated from the same embedded files.
