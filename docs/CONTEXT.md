# Tiger Go

Domain language for the Tiger Go project: a tool that deterministically forces AI agents to write Go that
is testable and free of whole classes of production bugs, with the specification in the source
and checked by the `tiger` analyzer. Terms here come from the Tiger Go Specification and
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
A directive stating a claim tiger proves, blocking once written: a loop variant
(`//tiger:variant`) or a function contract (`//tiger:requires`, `//tiger:ensures`). A variant has a
computed baseline, so its pin is optional where tiger can synthesize one; absence of a pin never
means the fact is absent. Effect and frame pins are removed (ADR-0026).
_Avoid_: declaration (pins are one kind of declaration, not the only kind)

**Placement**:
The rule binding a directive to the node it annotates: the node starting on the line after the
directive's comment group ends. One site reads it (collection, for the analyzers) and one site
writes it (insertion, for `tiger pin`); a pin written by the tool and a pin typed by hand are
indistinguishable afterward.

**Intent declaration**:
A directive stating something no analyzer can compute; stated first, enforced after.
Invariants and the `TS-X03` layer file are intent too, declared outside a directive.

**Escape hatch**:
A directive loosening one rule at one site, always carrying a reason; admitted only where no
in-subset code shape can accomplish the task. `//tiger:batched <reason>` is the only one, and it
waives only TS-M10 in `ioinloop`; ADR-0014 removed its waiver of the TS-S02 loop bound. Shape is
machine-checked, truth is human-reviewed, and every escape is counted against its package's budget
on every run, printing when the package goes over. `//tiger:openenum <reason>` is an escape too,
because tiger cannot check that an enum really is open.
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
The current value of something a pin can freeze: a synthesized loop variant. Not a rule:
registered in the facts table, printed only under `--show-facts` in pin syntax, never counted, and
what `tiger pin` freezes. Effect sets and frames were computed facts until ADR-0026.

**Effect set, frame** (removed):
The analyzer-computed summary of what a function does, and the set of locations it writes. ADR-0026
removed both, with their pins, the stdlib effects table, and the `effects` and `frames` analyzers,
because they missed every call through an interface or a library.

**`tiger changed`**:
The report, not a rule, that lists for each struct or free function the types and functions it
calls now that it did not call at a base revision. It replaces effect and frame pins (ADR-0026).
Decided, not built.

**Pattern**:
A runnable Go package under `patterns/<name>/` showing the correct shape where a rule stops short
and a real bug lived in the gap, printed by `tiger pattern <name>` (ADR-0025). Not a reusable
runtime package. Decided, not built.

**Variant**:
A ranking expression for a loop, required to strictly decrease on every back edge and be bounded
below, the standard termination argument. Synthesized by the analyzer where it can (fixed steps,
constant division and shifts, slicing by a part shown non-empty, struct fields, `&&` conditions),
pinned where it cannot.

**Fact**:
The `go/analysis` mechanism carrying one package's computed results to the packages that import
it; how transitivity crosses package boundaries within a module. In-memory
per run, never serialized.

**Registry**:
The closed set of every rule — custom rules bound to their analyzer and severity,
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
(`for it.Valid()`, `for rows.Next()`). `//tiger:batched` no longer waives its TS-S02 bound
(ADR-0014): a store loop restates the limit its query declares in its header, and a loop that
filters rows has a page limit and a scan limit.

**Stop case**:
A select case TS-S03/TS-C05 accept as ending a wait on shutdown: a receive from `ctx.Done()` on a
context that does not trace to `context.Background()` or `context.TODO()`, or a receive from or
send on a channel the package itself closes or sends on (ADR-0020). A channel's name or `struct{}`
element type does not count.
_Avoid_: shutdown channel (the name-based recognition ADR-0020 removes)

**Open enum**:
A named type marked `//tiger:openenum <reason>` whose vocabulary is deliberately extensible; switches
over it need a default arm but not exhaustiveness or `assert.Unreachable`. Recognized
same-package only — a switch in another package from the marked type is a documented known
miss — and governs only the custom default-arm check; the `exhaustive` auto rule needs its own
`//exhaustive:ignore` opt-out.

**Promotion**:
The one-line registry severity edit moving a tuned advisory rule to blocking, backed by trial
evidence on at least two real codebases; demotion is the same edit in reverse.

**Whole-program rule**:
A custom rule whose finding cannot be decided inside any one package because its evidence is
spread across packages that do not import each other (TS-A07, TS-A09). Its analyzer's
per-package pass exports a package fact describing what the package contributes; the finish
step reads every such fact and reports. Runs only under the tiger CLI.
_Avoid_: cross-package rule (facts already cross packages; the distinguishing property is the
finish step)

**Per-package rule**:
A custom rule decided entirely inside one `go/analysis` pass over one package, reading facts its
dependencies exported. Every rule before ENG-152.

**Finish step**:
The driver's call, once after every package has been visited, to each registered finish
function with every loaded package and every fact its analyzer exported. Driver policy, not
analyzer code: golangci-lint and `analysistest` have no equivalent.
_Avoid_: post-pass, module pass

**Invariant const**:
A package-level const of a named string type that some `assert.Invariant` or `assert.Violates`
call in the module takes as its ID. Defined by use, not by living in a package named `inv`; a
const of that type never passed anywhere is still one (and fails TS-A07 with zero references).

**Diagnostic prefix**:
The `path:line:col: TS-XXX:` head of a finding line — position rendered by the CLI, rule code
written first by the analyzer. A parsing contract: tooling keys on it and wording changes never
touch it. A `TS-D06` line is positioned at `tiger.budget.yaml:<line>` (the row) or
`tiger.budget.yaml` alone (no row); the counted findings under it carry their own prefixes.
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
> escape tiger has is `//tiger:batched`, because a provider without a bulk endpoint is a fact
> of the world the code can't restructure away, and it waives only the rule against IO inside a
> loop, never a loop bound. It still counts against the package's **budget** on every run,
> whether it waives anything or not, and prints as a blocking finding once the package is over. If you think the finding itself is wrong, that's a false
> positive on a **blocking** rule, which is a bug in the analyzer: file it, and the case lands in
> the **corpus** so it can't regress."

## Flagged ambiguities

- "surface" is overloaded: the specification's **surface** (TS-I01..I07, an injectable boundary
  for nondeterministic effects) is unrelated to **surface testing** (the testing philosophy used
  by this repo's tests). Qualify which one you mean.
- "linter" vs "analyzer": golangci-lint linters enforce auto rules; tiger analyzers enforce custom
  rules. Do not use the words interchangeably.
- "declaration" was used loosely during design for both pins and intent declarations — resolved:
  **pin** (a claim tiger proves) and **intent declaration** (stated fact, enforced) are distinct
  lifecycles under the umbrella term **directive**.
