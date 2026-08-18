# Assert Adoption Playbook

Two rules — `TS-S18` (no naked `panic` outside the assert package) and `TS-S08` (every closed-set
switch ends in `assert.Unreachable`) — assume the target codebase already has an `assert` package.
Most Go codebases do not; Go itself ships `panic` and nothing else. Both trials this wave ran
(querator, git-server) hit this precondition before either rule could produce an actionable finding,
and the two hit it to very different degrees. This document is the sequence for closing the gap, and
the evidence for deciding when it is worth building the two analyzers the gap currently blocks
(`assertdensity`, `invariantsymmetry` — see the end of this document; neither ships this wave).

## The sequence

Four steps, in dependency order. Each step's output is what the next step needs. Each is
independently mergeable — nothing here requires a big-bang rewrite.

### 1. Introduce the `assert` package

Per the spec ("The prerequisite: an assert package"): the companion `assert.go` is the whole
package, about a hundred lines, no dependencies, **enabled in production**. That last part is not
negotiable — an assertion compiled out only ever protected the test suite, which defeats the point.
Two implementation traps to build in from the start, not retrofit later: no formatted variant (Go
boxes variadic arguments at the call site before the callee tests anything, so `assert.Okf(ok,
"got %d", n)` allocates on every call including the ones that pass — put formatting behind an `if`
that only runs on failure), and a separate `//go:noinline` failure function (inlining has a budget,
and a fat failure path burns it on the hot path that never takes it).

Land this as one PR: the package, its own tests, nothing else changed. It compiles and does nothing
yet — no call site exists.

### 2. Route naked panics through it (`TS-S18`)

`forbidigo` on `^panic$` plus `paniccheck`'s assert exemption is what enforces this once the package
exists. Before that, every `panic(...)` in the tree is a `TS-S18` finding with no compliant rewrite
available. Work the list one call site at a time:

- A panic guarding an invariant the code believes cannot happen (an exhaustive-switch guard arm, an
  internal precondition) becomes `assert.Unreachable(...)` or `assert.Ok(cond, ...)` — same crash
  behavior, now routed through the one crash path the spec wants.
- A panic that is really signaling an *expected* operating condition (bad external input, a resource
  that legitimately might not exist) is not an assertion at all — it is a design bug independent of
  this playbook: it should return an `error` per `TS-E01`, not get relabeled `assert.Ok`.

The second category is why this step is not a mechanical find-and-replace: every panic needs the
"is this a bug in this code, or an expected condition laundered through panic" judgment call before
it gets touched. That judgment is exactly `TS-A02`'s "assert every precondition" reasoning, done once
per site instead of newly at review time forever after.

### 3. Give closed-set switches `assert.Unreachable` defaults (`TS-S08`)

Same shape, one level up. For a switch over a genuinely closed set — every case the type can hold is
already there — a missing or silent `default` is where an unhandled future value goes to disappear.
Add `default: assert.Unreachable(...)`, and the next variant that lands without updating this switch
now fails loud instead of silently falling through.

Not every switch that looks closed is: a type whose vocabulary is deliberately extensible (a wire
enum you expect to grow) wants a real catch-all default, not a crash. Mark that type `//tiger:openenum`
instead of reaching for `assert.Unreachable` — see `TS-S08` in the specification for the full
distinction and the separate opt-out `exhaustive` (the auto half of this rule) needs. Getting this
choice wrong in either direction is a real cost: an `assert.Unreachable` on a type that legitimately
grows crashes production on the first wire change; a plain default on a type that should be closed
hides the bug `TS-S08` exists to catch.

### 4. Declare the `inv` invariant vocabulary

Only after steps 1–3 land does this step have anything to attach to. Create the project-owned `inv`
package (see the specification's "The invariant vocabulary" section for the full pattern), name the
properties the codebase actually depends on, and replace ad hoc `assert.Ok(cond, "some string")`
call sites that concern a *named* property with `assert.Invariant(inv.SomeID, cond)` — at both sides
of the boundaries `TS-A03` cares about (validate before a write, validate again after the read that
should round-trip it).

This step is the one with no mechanical trigger — there is no analyzer finding that forces it, because
until `inv` exists there is nothing for `TS-A07`–`TS-A09` to check. It is deliberate design work, and
it is also the step that turns steps 1–3 from "stopped panicking incorrectly" into "assertions that
compound": `TS-A07` (every invariant referenced from at least two functions) and `TS-A08` (symmetric
boundary functions assert the same set) both need named invariants with real references before they
can report anything but "zero declared, zero to check."

## Worked example: querator — 30 findings, one blocker

Querator (ENG-148, `1fd1bb2`) has no `assert` package. Every `TS-S18` and `TS-S08` finding on it sat
behind step 1, all 30 of them:

- **`TS-S18`, 15 findings.** Naked `panic` calls throughout the tree. 5 of the 15 are already
  exhaustive-switch guard arms — the exact shape step 2 turns into `assert.Unreachable` with no
  behavior change, because they already crash on the unreachable case; they are just spelled with
  the wrong primitive.
- **`TS-S08`, 15 findings.** Switches missing an `assert.Unreachable`-shaped default. 3 of the 15
  are not mechanical: they are a legitimate catch-all dispatch and two log-and-degrade defaults,
  which would *change behavior* (crash instead of degrade) if step 3 were applied blindly — the
  judgment call the sequence above calls out. One of the 15 also surfaced a real bug: two storage
  backends (`internal/store/mongo.go:1422`, `postgres.go:1646`) drop `ActionItemMaxAttempts` with no
  default arm at all, silently, while the other two backends at least warn — exactly the defect class
  `TS-S08` exists to catch, found the moment the rule had something to check against.

30 findings, one prerequisite, one PR (step 1) that unblocks all of them. This is the shape the
playbook is written for: a codebase where the assert package is pure unlock — every downstream
finding was already true by spec, just unreachable until the package existed.

## Counter-example: git-server — 4 findings, and they barely count

Git-server (ENG-159, `321da03`) hit the same two rules and produced a different picture entirely:

- **`TS-S18`, 4 findings.** All four sit in the cross-adapter conformance MVP adapter — test-only
  fixture panics never linked into the server binary. Step 2's judgment call resolves all four the
  same way (they are test fixtures, not production code guarding an invariant), and none of them
  represents production risk today.
- **`TS-S08`, 14 findings.** Eleven are fail-closed or display-only defaults (auth switches that
  already default to deny or to the most restrictive level) — code that is already doing the safe
  thing, just not through `assert.Unreachable`. Two are real gaps worth fixing regardless of assert
  adoption (`writeHunk`'s missing default in `internal/graphapi/diff.go:333`, `packCode`'s invalid
  nibble in `internal/pack/writer.go:66`). One is the open-enum friction step 3 calls out by name
  (`PackReason`) — not a gap at all, a rule that needed the `openenum` vocabulary this wave adds.

The trial report's own verdict: "unlike querator (15+15 blocked on assert adoption), git-server
barely trips the assert-prerequisite rules." Same two rules, same lack of an `assert` package in
either codebase, and one codebase's gap is 30 findings deep while the other's is 4 test-only findings
that do not touch production. **The prerequisite is real, but whether it is worth paying is
codebase-dependent** — a codebase that already panics rarely and writes fail-closed defaults by habit
gets little from this playbook; a codebase that panics freely and treats switch defaults as
an afterthought gets a lot.

## When `assertdensity` / `invariantsymmetry` become worth building

Neither analyzer ships this wave (see the blueprint's Out of Scope). Both need a population of real
assertions to measure — `assertdensity` counts assertions per function and package (`TS-A01`,
`TS-A02`); `invariantsymmetry` pairs boundary functions and diffs their invariant sets (`TS-A08`).
Built against a codebase that has not run steps 1–4, both analyzers report the same finding steps 1–3
already report — "there is no assert package" — with more machinery and less clarity. Querator's 30
gated findings are the evidence: building a density rule on top of an unresolved `TS-S18`/`TS-S08`
gap does not produce new signal, it just restates the same 30 findings through a second lens.

The measurable state to cross before either analyzer is worth building, on a given target codebase:

1. `TS-S18` and `TS-S08` findings are resolved **without a behavior change** — naked panics are
   either `assert.Ok`/`assert.Unreachable` (bugs) or converted to `error` returns (expected
   conditions, per `TS-E01`), and closed-set switches carry a real `assert.Unreachable` default
   where one is warranted, `//tiger:openenum` where it is not.
2. The `inv` package exists, with at least a handful of invariant IDs, each referenced from real
   `assert.Invariant` call sites — not a stub with the pattern but nothing declared in it.

Only once both hold does `assertdensity` have assertions worth averaging instead of zero, and
`invariantsymmetry` have invariant sets worth comparing instead of an empty pair on every boundary
function. Recording this bar is this document's job; crossing it, on a given target codebase, is a
prerequisite this wave deliberately leaves to that codebase's own adoption work — querator has not
crossed it, so `assertdensity`/`invariantsymmetry` would report noise there today.

## What this playbook is not

- Not a mandate. Git-server shows a codebase can be compliant-by-habit without ever running this
  sequence; the playbook is for codebases where the trial evidence (or a `tiger check` run) shows the
  gap querator had.
- Not a promise that steps 1–3 are mechanical. Step 2 and step 3 both require a real judgment call at
  every site (bug vs. expected condition; closed vs. open vocabulary) — the playbook sequences the
  work, it does not remove the thinking.
- Not the deviation directive or the `TS-D06` ratchet. Neither ships this wave (see ADR-0005); this
  document assumes the wave-1.5 rule set as shipped.
