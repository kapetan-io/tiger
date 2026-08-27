# 8. Pin placement lives in one writer, not in suggested fixes

Date: 2026-08-26

## Status

Accepted

## Context

Writing a `//tiger:` directive into source requires deciding where it goes. The rule is already
fixed and already implemented once, on the reading side: a directive binds to the node starting on
the line after its comment group ends. One package owns that rule so a directive can never bind to
one node for one analyzer and a different node for the next.

`tiger pin` needs the same rule run backwards, for three verbs across two node kinds — effect and
frame directives on a function declaration, variant directives on a loop.

`golang.org/x/tools/go/analysis` offers `SuggestedFixes` on a diagnostic as the native structured
channel for source edits. Choosing it would mean each analyzer attaches its own insertion edit to
the fact it reports, with the command reduced to a generic applier. That path comes with
`analysistest.RunWithSuggestedFixes` for golden-file validation and is the mechanism golangci-lint's
`--fix` consumes, so it may hand that integration over for free.

It also spreads placement across three analyzers, against a codebase whose organizing rule is that
directive text has exactly one formatting site and directive attachment exactly one reading site.
And it makes every plain check run construct edits that nothing applies.

## Decision

We will implement one placement writer in the package that already owns directive attachment,
handling insertion for all three verbs. Analyzers keep reporting facts and gain no edit-building
responsibility.

Correctness is proven by a round trip rather than golden files: write a directive, re-parse the
file, collect directives, and assert the written one binds to the node whose fact produced it.

## Consequences

- The read and write sides of the attachment rule live in one package and are tested against each
  other, so they cannot drift apart.
- The round-trip assertion is stronger than a golden file. A golden file proves the output matches
  what someone accepted; the round trip proves the directive actually binds where intended.
- A fourth pinnable fact kind in a later wave reuses the writer instead of adding a fourth
  placement implementation.
- Check runs build no edits they discard.
- Per-analyzer validation of edits through the standard test harness is given up, and with it any
  free `--fix` support under golangci-lint.
- Adding `--fix` later means attaching suggested fixes as a second consumer of the same writer,
  which keeps placement in one site rather than moving it.
