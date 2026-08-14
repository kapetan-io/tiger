# 2. Keep analyzers driver-agnostic and severity in the driver

Date: 2026-08-12

## Status

Accepted

## Context

Tiger's custom rules must run identically under three drivers: the `tiger` CLI, golangci-lint via
module plugin, and the `analysistest` test harness. The Tiger Go Specification assigns each rule
one of three severities — blocking (fails the run), advisory (printed and counted, does not
fail), reported (annotation-only) — and some rules split severity by finding kind (TS-L10 blocks
a `defer` in a loop but is advisory on defer distance). The `go/analysis` diagnostic model carries
no severity field, so severity must be encoded somewhere. Encoding it inside each analyzer couples
run policy into twenty-plus passes and makes the plugin a port instead of a shim.

## Decision

We will implement every custom rule as a pure `golang.org/x/tools/go/analysis` pass that emits
diagnostics tagged with its rule ID and contains no severity, exit-code, or output-formatting
logic. Severity is defined once per rule in a rule registry; the driver applies it — partitioning
findings, computing exit codes, and formatting output. A rule with split severity registers each
half at its own level.

## Consequences

- Analyzers are portable across drivers unchanged.
- Changing a rule's severity is a one-line registry change, not an analyzer change.
- Every diagnostic must carry its rule ID, and a meta-test must enforce that the ID resolves in
  the registry — an unregistered analyzer cannot ship.
- A driver that does not consult the registry (a plain golangci-lint run) treats all findings
  uniformly; advisory and reported semantics are only guaranteed under the `tiger` CLI until the
  plugin configuration maps severities.
