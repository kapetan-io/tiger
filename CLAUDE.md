# Working on tiger

## Adding a pattern (best practice)

Tiger has two parts: blocking rules for what it can check exactly, and a collection of patterns for
the parts of a correct shape no rule can check (ENG-191, call 15). Before adding a pattern, or a
rule whose message should point to one, follow the entry condition and all five requirements below.

**Entry condition.** A pattern exists only where a tiger rule stops short and a real bug lived in
that gap: a trial finding, a filed ticket, or a bug an experiment reproduced. A shape that is merely
good Go, with no rule it backs up and no bug behind it, does not qualify.

**What makes a pattern more than a wiki that goes stale.** Every pattern meets all five:

1. **Each pattern is runnable code, not prose.** A small Go package showing the right shape, next to
   the broken shape it replaces. Tests demonstrate both, the way `experiments/call7` shows
   `StopDoneLoop` cleaning up every time and `EarlyExitLoop` skipping cleanup. Tests are the proof.
2. **CI keeps the patterns honest.** Every pattern must pass `tiger check` with zero findings and run
   its tests under `-race`. A pattern can't fall out of date with the rules, because the build fails
   first.
3. **Every pattern names the rules it backs up and the part those rules can't see.** For example,
   the shutdown pattern backs TS-C02, TS-S03 and TS-C05. Tiger can check how the loop starts and
   stops, but not whether every waiter also watches `done`. That missing check is exactly what the
   pattern teaches.
4. **Rule messages point to the pattern.** TS-S03's message ends with something like "see pattern
   `shutdown-loop`", so an agent that hits the finding reads the right shape in place of
   improvising one.
5. **Each pattern gives reviewers a short checklist.** It covers the parts tiger can't check, as
   yes-or-no questions an agent reviewer can answer while comparing a diff to the pattern. For the
   shutdown loop: does every caller waiting on the loop's reply also select on `done`? Does
   `Shutdown` close stop before it waits? Is there a test that calls `Shutdown` with an expired
   context?

Patterns are not reusable runtime packages. A package that encodes a shape is not added in place of
a pattern; following the pattern is the goal.

The reasoning and the first seven patterns are in
`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, call 15.
