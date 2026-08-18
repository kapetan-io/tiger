# Tiger Go

Domain language for the Tiger Go project: a restricted Go dialect whose specification lives in
the source, checked by the `tiger` analyzer. Terms here come from the Tiger Go Specification and
the tiger-check blueprints; use them exactly.

## Language

**Rule**:
A single numbered requirement in the Tiger Go Specification, identified as `TS-<area><number>`.
_Avoid_: check, lint, guideline

**Analyzer**:
A `go/analysis` pass this project writes to enforce one or more custom rules.
_Avoid_: linter (reserved for off-the-shelf golangci-lint linters)

**Auto rule**:
A rule enforced by an off-the-shelf golangci-lint linter configured in `config/golangci.yml`;
tiger never reimplements one.

**Custom rule**:
A rule only a tiger analyzer can enforce.

**Directive**:
A `//tiger:<verb>` source comment; the shared namespace for pins, intent declarations, and escape
hatches.
_Avoid_: annotation, pragma

**Pin**:
An optional directive that freezes an analyzer-computed fact (effect set, frame, variant) into a
blocking contract; absence of a pin never means the fact is absent.
_Avoid_: declaration (pins are one kind of declaration, not the only kind)

**Intent declaration**:
A directive stating something no analyzer can compute (invariants, restrictions, `hot`, `wire`,
`owner`, `openenum`); stated first, enforced after.

**Escape hatch**:
A directive loosening one rule at one site, always carrying a reason; admitted only where no
in-subset code shape can accomplish the task — still, through wave 1.5, `//tiger:batched
<reason>` alone. Wave 1 validated and surfaced it with nothing consuming it; wave 1.5 consumes it
in `ioinloop` (TS-M10) and, on the cursor shape only, in `boundedloop` (TS-S02 — see Cursor
shape). Shape is machine-checked, truth is human-reviewed, and every escape surfaces as a
standing advisory finding regardless of which rule it waives.
_Avoid_: suppression, nolint (the golangci mechanism, not ours)

**Severity**:
A rule's run-level consequence — **blocking** (fails the run), **advisory** (printed and counted,
does not fail), or **reported** (annotation-only). Defined once per rule in the registry, never
inside an analyzer.

**Registry**:
The closed set of every rule in the dialect — custom rules bound to their analyzer and severity,
auto rules bound to their golangci-lint linter and baseline settings — from which the binary,
docs, meta-tests, and the `tiger golangci` audit are derived; the single source of rule identity.

**Corpus**:
The `analysistest` packages that are a rule's executable specification: failure-mode case,
compliant rewrite, and marked known misses.

**Known miss**:
A corpus case documenting code an analyzer's coverage deliberately does not catch; the mechanism
that forbids silent scope cuts.

**Driver**:
Whatever runs analyzers — the tiger CLI, golangci-lint via the plugin, or `analysistest`.
Analyzers are driver-agnostic; run-level policy (severity, exit codes, output) belongs to the
driver.

**Wave**:
A build milestone grouping analyzers by what they need from the code (wave 1: single-package
AST/types, computed-only). Waves sequence the build; **chains** (from the spec) sequence rule
value — a chain completes across waves.

**Invariant vocabulary**:
The project-owned `inv` package pattern declaring invariant IDs that `assert.Invariant` references
and analyzers count (TS-A07..A09).

**Cursor shape**:
A loop whose condition is a boolean method call on an identifier that a method call advances
(`for it.Valid()`, `for rows.Next()`); finite only because the backing store is finite. The one
loop shape where `//tiger:batched` waives the TS-S02 bound.

**Shutdown channel**:
A channel TS-S03/TS-C05 recognize as a termination signal alongside `ctx.Done()`: element type
`struct{}` (closed-channel broadcast), or a name containing shutdown/stop/quit/done (the
shutdown-request shape). Select cases accept either recognition; a bare receive or send outside
a select is exempt only under the name recognition — a neutral-named `struct{}` channel there is
a completion wait, the missing-cancellation bug itself.

**Open enum**:
A named type marked `//tiger:openenum` whose vocabulary is deliberately extensible; switches
over it need a default arm but not exhaustiveness or `assert.Unreachable`. Recognized
same-package only — a switch in another package from the marked type is a documented known
miss — and governs only the custom default-arm check; the `exhaustive` auto rule needs its own
`//exhaustive:ignore` opt-out.

**Promotion**:
The one-line registry severity edit moving a tuned advisory rule to blocking, backed by trial
evidence on at least two real codebases; demotion is the same edit in reverse.

## Relationships

- A **Rule** is enforced by exactly one engine: an **Auto rule** by a golangci-lint linter, a
  **Custom rule** by an **Analyzer**.
- An **Analyzer** enforces one or more **Rules** and owns one **Corpus** per rule.
- The **Registry** binds each **Custom rule** to its **Analyzer** and **Severity**.
- A **Driver** runs **Analyzers**; only the driver applies **Severity**.
- A **Pin**, an **Intent declaration**, and an **Escape hatch** are the three kinds of
  **Directive**, distinguished by lifecycle.

## Example dialogue

> **Dev:** "The `boundedloop` **analyzer** flagged my retry loop — is there a directive that
> waives it?"
> **Domain expert:** "No — there is no `bounded` **escape hatch** in the tool. Give the loop an
> explicit cap and assert on exhaustion, or use the event-loop shape TS-S03 describes. The only
> escape in the dialect is `//tiger:batched`, because a provider without a bulk endpoint is a fact
> of the world the code can't restructure away — and unless your loop is a **cursor shape**, it
> waives nothing here either. It still stays visible as an **advisory** finding on every run,
> whether it waives anything or not. If you think the finding itself is wrong, that's a false
> positive on a **blocking** rule, which is a bug in the analyzer: file it, and the case lands in
> the **corpus** so it can't regress."

## Flagged ambiguities

- "surface" is overloaded: the specification's **surface** (TS-I01..I07, an injectable boundary
  for nondeterministic effects) is unrelated to **surface testing** (the testing philosophy used
  by this repo's tests). Qualify which one you mean.
- "linter" vs "analyzer": golangci-lint linters enforce auto rules; tiger analyzers enforce custom
  rules. Do not use the words interchangeably.
- "declaration" was used loosely during design for both pins and intent declarations — resolved:
  **pin** (computed fact, frozen) and **intent declaration** (stated fact, enforced) are distinct
  lifecycles under the umbrella term **directive**.
