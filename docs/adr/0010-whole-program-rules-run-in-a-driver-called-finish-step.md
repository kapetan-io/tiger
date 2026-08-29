# 10. Whole-program rules run in a driver-called finish step

Date: 2026-08-28

## Status

Accepted

## Context

Tiger's custom rules are `golang.org/x/tools/go/analysis` passes, one package at a time, and
they are meant to run identically under the tiger CLI, golangci-lint's module plugin, and the
`analysistest` harness. Facts carry one package's results to the packages that import it, and
that direction is the only one the framework offers.

Three rules need the opposite direction:

- TS-A07 and TS-A09 count references to an invariant constant. The constant lives in one
  package; the assertions and the violating tests live in the packages that import it, which
  run after it and never report back.
- TS-X01 counts implementations of an interface. The interface lives in one package; the
  implementing types live anywhere in the module.

By the time the declaring package has been visited, nothing that references it has run, and
once the referencing packages run, the declaring package is finished. No single pass sees both
sides. golangci-lint's runner and `analysistest` expose no hook that runs after the last
package.

The tension is between keeping every rule portable across all three drivers and shipping rules
that are, by construction, decidable only over the whole module. Two shapes were available: an
optional finish function attached to the existing rule registry entry, or a separate
program-level pass type with its own registry table. The second forks the one table the
corpus meta-tests iterate to prove that every registered rule is enforced and tested, and
duplicates name, documentation, and severity plumbing for three rules.

## Decision

We will split a whole-program rule into two halves. The per-package half stays a pure
`go/analysis` pass that exports a package fact describing what the package contributes. The
finish half is an optional function registered on the rule's existing registry entry, which the
tiger driver calls once after every package has been visited.

A finish function reports findings the same way passes do. A panic or error in one is an
operational failure: the run returns no findings.

The golangci-lint plugin returns the per-package halves unchanged and never calls a finish
function, so TS-A07, TS-A09, and TS-X01 are rules only `tiger check` reports.

## Consequences

- The per-package half of every rule remains portable across all three drivers; only the
  finish half is driver policy, and the analyzer still cannot tell which driver runs it.
- Under golangci-lint three rule codes are silently absent rather than wrong: the facts still
  export, nothing panics, and no finding fires.
- `analysistest` cannot exercise a finish function, so whole-program rules are tested by running small
  module corpora through the tiger driver instead, and the corpus meta-tests accept that second
  layout.
- The rule registry stays one table; a whole-program rule is an entry with one extra field, so
  "registered means enforced and tested" continues to hold from a single iteration.
- Determinism now depends on each finish function sorting its own output; the double-run diff
  test is the backstop.
- If golangci-lint ever exposes a module-level hook, the finish contract is the seam to wire it
  to; nothing about the rules themselves would change.
