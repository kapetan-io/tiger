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
The report, not a rule, that lists for each **function** the **calls added** and **calls removed**
between the merge base of a base revision and the working tree; it replaces effect and frame pins
(ADR-0026 and its amendment, ADR-0030).
_Avoid_: diff, effect report

**Owner**:
In a `tiger changed` report, the named type that holds a method where a call is written, or the free
function itself.

**Function header**:
In a `tiger changed` report, the line naming where calls are written: `<pkg>.<Type>.<method>` for a
method, `<pkg>.<function>` for a free function.

**Callee**:
In a `tiger changed` report, what a function calls: a function, type or interface in another package
(outside the module only if it passes the mixed filter), or a function of a different owner in the
same package whose own body calls another package.
_Avoid_: edge, target

**Call added** / **Call removed**:
One (function, callee) pair present at head and not at the merge base, or the reverse, printed as one
`+` or `-` line.
_Avoid_: new edge, `~` change

Relationships: a **function header** has one or more **calls added** or **calls removed** under it;
each names one **callee**; every function belongs to one **owner**.

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
element type does not count. TS-S23 narrows it for its own question: a channel the package sends on but
does not close is the stop case only when its case ends in a `return`.
_Avoid_: shutdown channel (the name-based recognition ADR-0020 removed)

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

**Command**:
One tiger subcommand's contract with the shell: the arguments after its name, two output streams
(`command.Streams`, stdout and stderr), and an exit code (0 clean, 1 findings, 2 operational
failure). Every domain package that owns a subcommand exports one function with this shape;
`cli.Run` only looks the name up.

**Usage**:
One command's entry in the command listing: its synopsis line (`tiger <name> [flags] <arguments>`)
and the description under it, which says what the command does and what its exit codes mean. The
package that owns the command owns its usage, and `tiger <name> -h` prints it above the flag
defaults.

**Command listing**:
What bare `tiger`, `tiger help` and an unknown command print: `usage:` followed by every command's
usage in a fixed order. `internal/cli` owns the order; each command's package owns its entry.

**CI workflow**:
Tiger's own continuous integration (CI) workflow, `.github/workflows/ci.yml`: the jobs that run on
every PR and the rules that skip jobs a change cannot affect. `internal/ci` holds only tests of it.

**Module (Go)**:
The Go module a run sits in: its module path, module-relative package paths, directory layout and
Go files. Read once per run; not the config file.

**Check run**:
Everything `tiger check` does from arguments to exit code: run setup, the driver call, the severity
partition, the TS-X03 and TS-T12 coverage findings, the ratchet, and output. `tiger budget` is the
same run printing only the ratchet.
_Avoid_: calling a rule a check (see Rule)

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
