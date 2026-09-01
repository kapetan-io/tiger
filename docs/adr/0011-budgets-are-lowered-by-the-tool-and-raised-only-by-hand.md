# 11. Budgets are lowered by the tool and raised only by hand

Date: 2026-08-28

## Status

Accepted

## Context

Tiger's advisory findings print on every run and never fail one. Two of them stand on purpose:
every escape directive and every skipped test. Nothing bounds how many a package carries, so the
accounting that makes an escape hatch honest has no teeth, and the printout is a warning in a
tool whose output is meant to be a verdict.

The Tiger Go Specification's TS-D06 answers this with a per-package budget file checked in and
reviewed like code: numbers that may only decrease. "A global threshold gets raised at 2am before
a release; a ratchet cannot be raised without a commit that says so."

A tool that writes that file has to decide three things:

- Whether it may ever increase a number. The convenient form is a flag that resets a package's
  budget to its current count after a deliberate change.
- Whether it should detect a raise itself, which means reading git history to compare against the
  previous committed value.
- What a package with findings and no row means: zero, or unchecked.

The dialect's primary consumer is an AI coding agent, and an agent offered a cheaper path to a
green build than conforming will take it. A raise flag is that path: every overrun, including a
real regression, clears with one command that touches no logic. A mechanism is judged by the
cheapest thing it makes possible, not by its intended use.

## Decision

We will give the budget writer a lowering-only contract. `tiger budget --write` can shrink an
existing row, create a row for a package that has none, or delete a row that reaches zero; it has
no operation that raises an existing number.
A raise is a hand edit that appears in a pull request diff for a reviewer to rule on. Tiger does
not read git to detect raises; review is the gate.

A package with advisory findings and no budget row has a budget of zero and fails the run.

Counted findings under budget print nothing. On overrun the package's counted findings print as
blocking lines. The budget file is where a reader sees which deviations a package carries.

## Consequences

- The refusal is structural. No caller can express a raise, so the guarantee cannot erode through
  a later flag added in good faith.
- The cheapest parseable path out of an overrun is fixing the findings or a human-authored,
  reviewable budget edit.
- The first run after adoption fails on every package that carries an escape or a skipped test
  until `tiger budget --write` records the current counts once.
- Deleting a row by hand and rerunning the writer recreates it at the current count. In the diff
  that reads as a raise from absent to N; review must catch it, because the tool cannot
  distinguish first adoption from a deleted row without git.
- Check output is binary: blocking lines and a verdict, or nothing. Slack is visible only as a
  number in the budget file's diff, so a budget that drifts above reality is caught in review of
  that file, not in a run's output.
- Budgets are enforced by the `tiger` CLI only. The golangci-lint plugin has no end-of-run hook
  to aggregate counts across packages and no warning tier to map to, so it reports every counted
  finding as an issue; a repo that carries any budgeted debt needs the CLI.
