# 22. Tiger has no baseline and recommends no new-code filter

Date: 2026-10-12

## Status

Accepted

## Context

Some linters let an adopter record today's findings in a file and fail only on new ones. Tiger has
no such file for blocking rules. A first run prints 396 rule findings on querator and 323 on
git-server, so an existing codebase cannot adopt tiger a piece at a time. It has to fix everything
or turn analyzers off
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, call 9).

ADR-0011 refused the same mechanism for advisory findings, because a file that raises a limit is
the cheapest path to green an agent can find. The argument holds for blocking findings too: a
baseline gives an agent a command that turns any regression green.

`config/golangci.yml` tells adopters to run with `--new-from-rev=origin/main`, which is a baseline
by another route. `experiments/newfromrev` shows what that filter does to tiger's findings. With
`--new-from-rev=HEAD~1` the plugin drops the finding on the line the branch didn't touch and prints
nothing about it. With `--new-from-rev=HEAD` a repository with two blocking findings passes. Run
through a symlinked path, it drops every finding, including the new one. `tiger check` reads no git
history and reports both.

## Decision

Tiger gets no baseline mechanism and does not recommend one. `config/golangci.yml` drops its line
telling adopters to use `--new-from-rev=origin/main`.

An adopter who chooses golangci-lint's own new-code filters (`--new-from-rev`, `new-from-merge-base`,
`new-from-patch`, `issues.new`) may use them, and tiger does not check for them. That is the
adopter's call, not tiger's default.

## Consequences

- A codebase adopts tiger by fixing what it reports. Tiger's stated target is new AI-written code,
  where this cost does not arise. For an existing codebase the cost is real: hundreds of findings on
  the first run.
- `tiger check` stays the run that sees every finding, since it reads no git history. An adopter
  who filters golangci-lint's output to new code still has `tiger check` in CI.
- If the project later wants brownfield adoption, that needs its own ADR, with a design that can
  only ever lower the numbers, like ADR-0011's budget file.
