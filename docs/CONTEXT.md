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

**Placement**:
The rule binding a directive to the node it annotates: the node starting on the line after the
directive's comment group ends. One site reads it (collection, for the analyzers) and one site
writes it (insertion, for `tiger pin`); a pin written by the tool and a pin typed by hand are
indistinguishable afterward.

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
A rule's run-level consequence — **blocking** (fails the run) or **advisory** (counted per
package against the budget file: prints nothing under budget, prints as blocking under a `TS-D06`
line on overrun; reserved for the standing notices the specification names and ADR-0006's
trial). There is no third level: a rule either blocks or it is not a rule (ADR-0012), and no
warning tier: check output is a verdict. Defined once per rule in the registry, never inside an
analyzer.
_Avoid_: reported (the removed tier; computed facts are not rules and carry no severity)

**Fact (computed)**:
The current value of something a pin can freeze — an effect set, a frame, a synthesized loop
variant. Not a rule: registered in the facts table, printed only under `--show-facts` in pin
syntax, never counted, and what `tiger pin` freezes.

**Effect set**:
The analyzer-computed summary of what a function does, over the closed lattice `alloc`, `io(q)`,
`block`, `panic`, `rand`, `time`, `mutate(x)`, `spawn`; the empty set is purity, spelled `none`
in pin syntax. Transitive by construction — a function's set includes everything reachable
beneath it.

**Frame**:
The analyzer-computed set of locations reachable from a function's parameters and receiver that
the function writes. A pinned frame makes writes outside it blocking.

**Variant**:
A ranking expression for a loop, required to strictly decrease on every back edge and be bounded
below — the standard termination argument. Synthesized by the analyzer where it can (linear
integer expressions over locals), pinned where it cannot.

**Fact**:
The `go/analysis` mechanism carrying one package's computed results (effect sets, frames) to the
packages that import it; how transitivity crosses package boundaries within a module. In-memory
per run, never serialized.

**Stdlib effects table**:
The curated, committed mapping from standard-library functions to their effects (`os.(*File).Write`
→ `io(disk)`); the source of built-in io qualifiers. Data with tests; its coverage gaps are
known-misses.

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

**Whole-program rule**:
A custom rule whose finding cannot be decided inside any one package because its evidence is
spread across packages that do not import each other (TS-A07, TS-A09, TS-X01). Its analyzer's
per-package pass exports a package fact describing what the package contributes; the finish
step reads every such fact and reports. Runs only under the tiger CLI.
_Avoid_: cross-package rule (facts already cross packages; the distinguishing property is the
finish step)

**Per-package rule**:
A custom rule decided entirely inside one `go/analysis` pass over one package, reading facts its
dependencies exported. Every rule before ENG-152, plus `restrictions` and `closedworld`.

**Finish step**:
The driver's call, once after every package has been visited, to each registered finish
function with every loaded package and every fact its analyzer exported. Driver policy, not
analyzer code: golangci-lint and `analysistest` have no equivalent.
_Avoid_: post-pass, module pass

**Invariant const**:
A package-level const of a named string type that some `assert.Invariant` or `assert.Violates`
call in the module takes as its ID. Defined by use, not by living in a package named `inv`; a
const of that type never passed anywhere is still one (and fails TS-A07 with zero references).

**Test double (TS-X01)**:
A type declared in a `_test.go` file. It never counts as an implementation for `singleimpl`; a
fake that must count lives in a non-test package.

**Restriction set**:
The parsed `//tiger:restrict` declaration of a package: the `closed-dispatch`, `no-reflect`, and
`imports(...)` axes. Absent directive or axis means the axis takes its stated default and is not
checked.

**Diagnostic prefix**:
The `path:line:col: TS-XXX:` head of a finding line — position rendered by the CLI, rule code
written first by the analyzer. A parsing contract: tooling keys on it and wording changes never
touch it. The CLI's ` [advisory]` marker is a separate print-time annotation inserted after the
code on advisory lines, not part of the prefix.
_Avoid_: header, tag

**Message body**:
The prose after the diagnostic prefix, written for a reader new to tiger: what the code does,
what tiger expected, the edit to make. Carries exactly one rule code (the prefix's), spells
directives literally, and uses no internal vocabulary without a gloss.
_Avoid_: description, explanation

**Config file**:
The committed `tiger.yaml` at the module root holding facts about the world for the analyzers
(participle nouns, IO packages). Every entry is a value, a reason, and an optional package
scope, and is reviewed like code. An entry widens what an analyzer detects or corrects a
dictionary; no entry names a function, file, or site to exempt.
_Avoid_: allowlist file, settings (the golangci term), suppression list

**Budget file**:
The committed `tiger.budget.yaml` at the module root: one integer per package per advisory rule,
written by `tiger budget --write` and only ever lowered by it. A missing row is a budget of zero.
_Avoid_: baseline (the golangci term), threshold

**Budget**:
The number of advisory findings of one rule a package is allowed to carry; the count of the
rule's findings in that package must not exceed it.

**Ratchet**:
The TS-D06 rule that budgets only decrease. `tiger check` enforces count ≤ budget and fails the
run on an overrun; the tool never raises a number; a raise is a hand edit visible in review.
_Avoid_: quota, cap

**Slack**:
A budget above its package's current count. Silent in check output; visible only as the number
in the budget file's diff, lowered by `tiger budget --write`.

## Relationships

- A **Rule** is enforced by exactly one engine: an **Auto rule** by a golangci-lint linter, a
  **Custom rule** by an **Analyzer**.
- An **Analyzer** enforces one or more **Rules** and owns one **Corpus** per rule.
- The **Registry** binds each **Custom rule** to its **Analyzer** and **Severity**.
- A **Driver** runs **Analyzers**; only the driver applies **Severity**.
- A **Whole-program rule** has a per-package pass and a **Finish step**; a **Per-package rule** has
  only the pass. Only the tiger CLI runs finish steps.
- A **Pin**, an **Intent declaration**, and an **Escape hatch** are the three kinds of
  **Directive**, distinguished by lifecycle.
- **Placement** binds every **Directive** to a node; `tiger pin` writes a **Pin** at that
  placement, and never edits or removes one that is already there.
- The **Config file** feeds **Analyzers** through the **Driver**; the **Budget file** feeds only
  the `tiger` CLI driver, which applies the **Ratchet** to **Advisory** counts.

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
