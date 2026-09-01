# 12. A rule either blocks or does not exist

Date: 2026-08-28

## Status

Accepted

## Context

Tiger is a Go subset for AI-written mission-critical software. Its value is a binary verdict:
the code is in the dialect or it is not, and a change to a frozen fact is caught. Warnings —
findings a reader may weigh and set aside — are golangci-lint's job, not tiger's.

The registry carried three severities. Blocking fails the run. Advisory prints and counts but
never fails; the specification reserves it for the standing notices it names (every escape
directive, every skipped test) and for ADR-0006's time-boxed trial of a heuristic rule.
Reported printed only under `--show-facts` and never counted: it was used for computed facts
(effect sets, frames, synthesized loop variants), and the cross-package blueprint proposed using
it for the TS-P02 precision bound as "a number, not a finding".

A reported rule is a rule nobody has to act on. Once the tier exists, every future rule whose
finding is awkward to fix has a place to go that is neither a verdict nor absence. The tier also
blurred two unlike things: a fact is the current value of something a pin can freeze, while TS-P02
weakened is a contract violation — a package claims closure its dependencies do not support.

## Decision

The severity model has two values: blocking and advisory. Reported is removed from the registry,
the CLI, the plugin, and the specification, and is not an option for any future rule.

Computed facts are not rules. They move to their own registry table (`rules.Facts`), carry no
severity, print only under `--show-facts`, and feed `tiger pin`. The CLI and the plugin resolve a
diagnostic category through the rules table first and the facts table second; a category in
neither is an operational error.

TS-P02 registers blocking. Its message names the edit: declare the axis on the dependency, or
drop the claim.

Advisory stays exactly as the specification defines it — the accounting channel ADR-0006 and
TS-D06 need — and is not a home for rules that would otherwise be warnings. Ratchet-counted
accounting is ENG-153's.

## Consequences

- A registered rule always changes the exit code or is one of the specification's named standing
  notices. There is no third outcome to design toward.
- A package that claims a restriction axis and imports a third-party package fails TS-P02 until
  it drops the claim: third-party code declares nothing and is the weakest on every axis. That is
  the incentive TS-D01 already sets.
- `--show-facts`, `tiger pin`, and the three fact categories are unchanged in behavior; only their
  classification moved. The plugin still drops them.
- The corpus meta-tests resolve fact categories through the facts table, so a fact category
  registered as a rule — or the reverse — fails the coherence test.
