# Cross-Package Analyzers Blueprint

Linear: [ENG-152](https://linear.app/kapetan-io/issue/ENG-152/cross-package-analyzers).

## Objective

Wave 1 and the SSA wave (ENG-147, ENG-150) shipped every rule a single package can decide, plus
the facts plumbing that lets one package's computed results reach the packages importing it.
Five rules in the Tiger Go Specification still have no analyzer because their evidence lives in
more than one package:

| Rule | Analyzer | What it needs from other packages |
| --- | --- | --- |
| TS-A07 every invariant asserted in two functions | `invariantrefs` | references to a const declared elsewhere |
| TS-A09 every invariant has a violating test | `invariantnegative` | `assert.Violates` calls in other packages' tests |
| TS-P01 declared package restrictions hold | `restrictions` | nothing (own imports); P02 needs dependencies' declarations |
| TS-K03 no dynamic dispatch where closure is claimed | `closedworld` | nothing beyond the package's own SSA and its declaration |
| TS-X01 no interface with exactly one implementation | `singleimpl` | implementing types anywhere in the module |

This wave ships those five and the one mechanism they share: a **finish step** the tiger driver
runs once after every package has been visited, where an analyzer that exported per-package facts
reads all of them back and reports on the whole module. Building it also settles a question the
SSA wave left open in prose only: what fact propagation looks like under the golangci-lint plugin,
now proven by a two-package smoke fixture run through the real custom binary.

`surfaces` (TS-I01/I03/I05/I07) and `surfacediff` (TS-R02, TS-P03) were on the deferred list
with these five and are split into their own tickets. `surfaces` cannot be built: the
specification's declaration table lists "surface fault set" as an intent declaration with no
directive form, and the directive vocabulary has no `surface` verb — that is a language
extension, not an analyzer. `surfacediff` is a checked-in artifact with an update mode and a
semantic-version decision the specification never wrote down; the wave-1 blueprint already called
it its own campaign. TS-P03 (weakening a restriction set blocks) moves with `surfacediff` because
only the checked-in file knows what the previous declaration was.

## Mental Model

Two kinds of custom rule now exist, distinguished by where the finding is decided:

- A **per-package rule** decides every finding inside one `go/analysis` pass over one package,
  reading facts its dependencies exported. Every rule before this wave is one. `restrictions` and
  `closedworld` are per-package rules.
- A **whole-program rule** cannot decide inside any single package because the evidence is spread
  across packages that do not import each other. Its analyzer has two halves: the per-package pass
  exports a fact describing what this package contributes (which invariants it asserts, which
  interfaces its types implement), and the finish step reads every such fact and reports.
  `invariantrefs`, `invariantnegative`, and `singleimpl` are whole-program rules.

The direction of facts is the whole reason the split exists. `go/analysis` moves facts from a
dependency to the packages that import it. An invariant is declared in `inv` and asserted in the
packages importing `inv`; by the time those packages run, `inv` has already been visited and can
never learn what referenced it. The finish step is the one place that sees both.

A whole-program rule runs only under the tiger CLI. golangci-lint's runner has no
end-of-module hook, and `analysistest` has none either. Under the plugin the per-package half
still runs and still exports its facts, so nothing panics and nothing is silently wrong; the
rule's findings are absent, and the plugin documentation names the three rule codes that only
`tiger check` reports.

## Core Design Principles

1. **Exact rules only.** Every finding in this wave names a concrete edit and is decidable from
   syntax, types, and SSA. Nothing here enters the ADR-0006 advisory trial. All five rules
   register blocking except the TS-P02 precision bound, which is reported (a number, not a
   finding).
2. **Absence of a declaration is never a finding.** A package with no `//tiger:restrict` makes
   no claim: `restrictions` checks nothing in it, `closedworld` is silent in it. Mirrors the pin
   model one level up (TS-P01).
3. **Analyzers stay driver-agnostic; the finish step is driver policy.** The per-package half of
   every analyzer is a pure `go/analysis` pass (ADR-0002). The finish half is a separate function
   the tiger driver chooses to call. No analyzer knows which driver runs it. ADR-0010 records the
   carve-out; ADR-0002 stands for every per-package rule.
4. **Match by shape, not import path.** Adopters copy the `assert` package into their own tree
   (`internal/assert`), so `assert.Invariant` is recognised by package name `assert` and function
   name, as `compoundcond` and `paniccheck` already do. An invariant is any const whose type is
   passed as the ID of some `assert.Invariant` or `assert.Violates` call in the module — package
   name is irrelevant.
5. **Messages follow ADR-0009 from the first draft.** Every template below is one line, names
   the edit, carries one rule code, and uses none of the banned vocabulary. The corpus meta-test
   enforces the mechanical half.

## Correctness Constraints

### State Invariants

1. **Every finding is positioned in the module under check, at the declaration the edit
   applies to.** TS-A07/A09 report at the invariant const; TS-X01 at the interface type; TS-P01
   at the offending import spec; TS-K03 at the call. Violated by a finish-step finding with a
   zero position or a position in a dependency outside the module. Enforcement: the finish step
   asserts every diagnostic's position resolves to a file in the loaded module before it is
   appended.
2. **A fact is exported only by the package that owns the described entity or by the package
   holding the evidence.** Reference and implementation facts are package facts of the package
   holding the reference or the implementing type, never object facts on an imported const or
   interface (the driver asserts ownership on export; golangci's runner does the same).
3. **Finish-step output is a function of the loaded packages only.** No map iteration order,
   visit order, or fact insertion order reaches the output. Enforced by the existing double-run
   determinism tests, extended to a fixture that exercises every whole-program rule.

### Behavioral Constraints

1. **Never present partial results as a complete run.** A panic or error in any finish step
   returns an operational error from `driver.Check` (exit 2), exactly as a panicking pass does
   today. Findings from other rules are not returned alongside.
2. **Never fire on a package that made no claim.** No `//tiger:restrict` means no TS-P01, no
   TS-K03. A module with no invariant consts produces no TS-A07/A09.
3. **Never count a `_test.go` type as an implementation for TS-X01, and never count a
   `assert.Violates` outside `_test.go` for TS-A09.** The file suffix is the boundary; it is
   exact and it is what the message tells the reader to change.
4. **Under the golangci-lint plugin, whole-program rules produce no findings and no errors.**
   The plugin's `BuildAnalyzers` returns the per-package halves unchanged.

Concurrency: none — a run is single-threaded per package, as today. Reversibility: no persistent
state is written; every finding recomputes from source. Partial failure: covered by behavioral
constraint 1.

## Acceptance Criteria

1. `tiger check` on `examples/ledger` exits 0. Deleting one `assert.Invariant` call from
   `header.go` makes it exit 1 with one TS-A07 finding at `inv/inv.go` naming the const and the
   one remaining function. Deleting `TestCorruptHeaderViolatesChecksum` produces one TS-A09
   finding at the const.
2. A fixture module with an interface implemented once by a non-test type exits 1 with TS-X01 at
   the interface; adding a second implementation in a non-test file exits 0; adding it in a
   `_test.go` file instead still exits 1.
3. A fixture package declaring `//tiger:restrict no-reflect, imports(internal/domain/...)` that
   imports `reflect` and a module path outside `internal/domain/` exits 1 with two TS-P01
   findings, one per import spec. The same package with a stdlib-only import set and no
   directive exits 0.
4. A fixture package declaring `closed-dispatch` with an interface method call whose receiver is
   a parameter exits 1 with TS-K03 at the call; the same call after the receiver is built from a
   single concrete value in the same function exits 0.
5. `tiger check --show-facts` prints one TS-P02 line per (package, weakened axis) — a package
   weakened on two axes by two different dependencies prints two lines — each naming the
   weakest transitive dependency on that axis; a second run prints identical bytes.
6. `TestEveryRegisteredRuleHasCorpus` and `TestEveryCorpusMessageFollowsTheStyleGuide` pass with
   the five new analyzers registered; the corpus for each whole-program rule runs through the
   finish step, not `analysistest` alone.
7. CI's `plugin-smoke` job runs the real `golangci-lint custom` binary over a two-package
   fixture and greps a TS-F02 finding that can only fire if the effects fact crossed the import,
   plus a TS-P01 finding; it asserts no TS-A07, TS-A09, or TS-X01 line appears (the plugin does
   not run finish steps). The fixture contains an invariant const asserted in exactly one
   function with no violating test and an interface with exactly one non-test implementation,
   and a CLI test asserts `tiger check` on the same fixture exits 1 with TS-A07, TS-A09, and
   TS-X01 — so the plugin run's silence on those codes is meaningful, not vacuous.
8. `tiger check ./...` on tiger's own tree exits 0 after the five analyzers register, with any
   real findings fixed in the same change.

## Scope

### In Scope

- Analyzers `invariantrefs` (TS-A07), `invariantnegative` (TS-A09), `restrictions` (TS-P01
  blocking, TS-P02 reported), `closedworld` (TS-K03), `singleimpl` (TS-X01), each with a corpus.
- The finish step in `internal/driver`, its registry field, its corpus harness, and its
  determinism and panic-containment tests.
- The `restrict` argument grammar in `internal/directive`, with malformed arguments reported as
  TS-L09 by `directives` (the SSA wave's contract for pin verbs, extended to this intent verb).
- Two-package plugin smoke fixture and CI assertions.
- Plugin and CLI documentation naming the CLI-only rule codes.
- ADR-0010: the finish step as a driver-side extension (written with this blueprint).
- Specification amendments: TS-A07 "distinct functions" definition, TS-X01 test-double
  definition, TS-P01 grammar, the "CLI-only" enforcement note on the three whole-program rules.

### Out of Scope / Non-Goals

- `surfaces`, `surfacediff`, TS-P03, TS-I03 implementation counting — follow-up tickets.
- TS-A08 `invariantsymmetry` — pairs by naming convention; a heuristic, and on the wave-1.5
  list.
- Running finish steps under golangci-lint. There is no hook; the plugin documents the gap.
- A serialized fact cache or any cross-module analysis. Facts stay in-memory per run; a
  third-party package with no declaration is the weakest on every axis (TS-P02).
- Layering (`TS-X03`) — `depguard` territory; the restrict directive's `imports(...)` axis is
  allow-listing within the module, not the layer file.
- Devirtualization through function returns, struct fields, or across packages for TS-K03. A
  receiver that is not built from a single concrete value inside the function is a finding —
  that is the rule, not a miss.

## Dependencies and Constraints

- Go 1.26, `golang.org/x/tools` v0.49.0, golangci-lint v2 plugin mechanism — unchanged.
- Builds on `internal/driver`'s facts store (object and package facts, both implemented and
  tested), `internal/directive`'s per-verb grammars, `internal/rules` registry and meta-tests.
- ADR-0002 (driver-agnostic analyzers), ADR-0006 (advisory bar), ADR-0009 (message standard).

User stories are omitted: developer-infrastructure work with one consumer type, and the
correctness constraints and acceptance criteria are already at story granularity.

---

## Functional

### Rule behavior

**TS-A07 (`invariantrefs`).** For every invariant const, count the distinct named functions
(methods included; closures fold into their enclosing function; test functions count) whose body
calls `assert.Invariant` with that const as ID, across every package in the module including
test variants. Fewer than two is a finding at the const.

**TS-A09 (`invariantnegative`).** For every invariant const, require at least one
`assert.Violates(<const>, ...)` call in a `_test.go` file anywhere in the module. None is a
finding at the const.

An **invariant const** is a package-level const whose declared type is a named type with
underlying `string` and which is passed as the first argument to any `assert.Invariant` or
`assert.Violates` call in the module. A const of that type that is never passed anywhere is still
an invariant (its type is): it gets a TS-A07 zero-reference finding, which is the "misfiled or
under-defended" case the rule exists for. A const of an unrelated string type that no assert call
ever names is not an invariant and is ignored.

**TS-P01 (`restrictions`).** In a package whose doc comment carries `//tiger:restrict`, parse
the axes and check each declared one against the package's own file set:

- `no-reflect`: no file imports `reflect`.
- `imports(p1, p2, ...)`: every non-stdlib import path is inside the module and matches one
  pattern. Patterns are module-relative paths; a trailing `/...` matches the subtree. Stdlib is
  always allowed (the TS-D01 baseline the axis extends).
- `closed-dispatch`: no check in `restrictions`; it is the claim `closedworld` enforces.

One finding per offending import spec. A second `//tiger:restrict` in the same package is a
TS-P01 finding at the second directive. Absent directive, absent axis: no finding.

**TS-P02 (`restrictions`, reported).** Each package with a declaration exports a package fact
carrying its declared axes. A package's bound on each axis is the weakest value among its own
declaration and every transitively imported module package's declaration (missing declaration is
weakest: open dispatch, stdlib-only baseline unclaimed, reflect permitted). Each axis is a
boolean, claimed or unclaimed; two present `imports(...)` declarations with different patterns
are both "claimed" and never compared against each other. Stdlib packages are ground and never
weaken. Each weakened axis is reported as its own TS-P02 line at the `package` clause under
`--show-facts`, naming the first dependency (in stable order) that sets it; lines for one package
are ordered closed-dispatch, no-reflect, imports. An axis that is not weakened prints nothing.
One axis per line keeps every line inside the 160-character preference regardless of how many
dependencies weaken the package.

**TS-K03 (`closedworld`).** In a package declaring `closed-dispatch`, every SSA call through an
interface (`CallCommon.IsInvoke()`) must have a receiver that resolves, walking backward through
`Phi`, `ChangeInterface`, and local `Store`/`Load` pairs within the function, to exactly one
`MakeInterface` of one concrete type. Anything else — a parameter, a field, a call result, a
global, two different concrete types meeting at a `Phi` — is a finding at the call. Calls on
interfaces from the standard library (`error.Error()`, `io.Reader.Read`) are not exempt; the
package claimed closure.

**TS-X01 (`singleimpl`).** For every interface type declared in a non-test file of a module
package, count the named types declared in non-test files of module packages whose method set
(as `T` or `*T`, counted once) satisfies it, excluding the empty interface and interfaces with no
methods. Exactly one implementation is a finding at the interface, naming the implementation.
Zero is not a finding (the interface is a contract with no implementer yet; TS-X01 is about
speculative indirection, not dead code). Interfaces declared in `_test.go` files are ignored.

### Message templates

Bodies are one line, at most 160 characters after the prefix in the corpus, each ending in the
edit. Identifiers are interpolated; `<pkg>.<Name>` is the qualified const or type.

```
TS-A07: invariant inv.HeaderSize is declared but no function asserts it — add assert.Invariant(inv.HeaderSize, ...) in two functions, or delete the declaration
TS-A07: invariant inv.HeaderSize is asserted only in encodeHeader — assert it in a second function, or delete the declaration
TS-A09: no test violates invariant inv.HeaderSize — add a _test.go function that calls assert.Violates(inv.HeaderSize, func() { ... })
TS-P01: package ledger imports github.com/acme/x, which its //tiger:restrict imports(...) list does not allow — remove the import or add the path to imports(...)
TS-P01: package ledger declares //tiger:restrict no-reflect but imports reflect — remove the reflect import or drop no-reflect from the directive
TS-P01: package ledger has two //tiger:restrict directives — keep one, merging the axes into a single comma-separated list
TS-P02: package ledger is checked with open dispatch because it imports pkg/store, which declares nothing — nothing to fix; add //tiger:restrict closed-dispatch to pkg/store to tighten it
TS-K03: s.Write is called through interface Storage in a package that declares //tiger:restrict closed-dispatch — call the concrete type's method, or drop closed-dispatch
TS-X01: interface Storage has one implementation, diskStorage — use diskStorage directly and delete the interface, or add a second implementation outside _test.go files
```

Malformed `restrict` arguments (unknown axis, unbalanced parentheses, empty `imports()`) are
TS-L09 findings from `directives`, naming the offending token, as the SSA wave defined for pin
verbs.

### Severity and registry

| Category | Rule | Analyzer | Severity |
| --- | --- | --- | --- |
| `TS-A07` | TS-A07 | `invariantrefs` | blocking (whole-program) |
| `TS-A09` | TS-A09 | `invariantnegative` | blocking (whole-program) |
| `TS-P01` | TS-P01 | `restrictions` | blocking |
| `TS-P02-facts` | TS-P02 | `restrictions` | reported |
| `TS-K03` | TS-K03 | `closedworld` | blocking |
| `TS-X01` | TS-X01 | `singleimpl` | blocking (whole-program) |

## Architecture

### The finish step

`internal/driver` gains one concept: after the package loop in `Check`, and only when every
pass succeeded, the driver calls each registered finish function once. Contract:

- **Input.** Every loaded package in the same stable topological order the passes used, with
  its `types.Package`, `TypesInfo`, syntax, and `Fset`; and read access to every fact the owning
  analyzer exported during the run (package facts enumerated in insertion order, object facts
  likewise). Nothing else: no results cache, no other analyzer's facts.
- **Output.** Diagnostics in the same shape passes emit (`analysis.Diagnostic` with category,
  position, message), merged into the same findings slice before relativization and sorting.
  Position must resolve inside the loaded module (state invariant 1).
- **Failure.** A returned error or a panic becomes `Check`'s error; no findings are returned.
- **Identity.** A finish function belongs to exactly one analyzer and is registered next to it
  in `CustomRule` as an optional field on the rule entries that need it. The registry stays
  one table; `Analyzers()` is unchanged; a new `Finishers()` accessor returns the ordered set.
  The corpus meta-test's "registered means enforced" iteration is unchanged.
- **Determinism.** The driver passes packages and facts in insertion order; a finish function
  that builds maps must sort before emitting. The double-run test on the whole-program fixture
  is the backstop.

The plugin's `BuildAnalyzers` returns `rules.Analyzers()` as today; it never sees finishers.
`analysistest` never sees them either, which is why whole-program corpora need the harness below.

**Rejected alternative:** a separate `ProgramPass` type outside `go/analysis` with its own
registry table. It forks the registry the meta-tests iterate and duplicates name/doc/severity
plumbing for three rules. The optional-field shape adds one field and one accessor.

### Fact shapes (contracts; exact structs are the implementor's)

- `invariantrefs` / `invariantnegative` share one per-package pass and one package fact: the
  set of (invariant const identity, referencing function identity, kind ∈ {assert, violates},
  in-test-file) tuples the package contributes, plus the set of invariant consts it declares.
  Identities are `(package path, objectpath)` so they survive the `pkg [pkg.test]` variant
  seam the facts store already handles. Two analyzer packages, one shared internal library
  (`internal/analyzers/internal/invariants`) that both `Run`s call; each analyzer exports its
  own fact type because `go/analysis` scopes facts per analyzer.
- `singleimpl`: a package fact listing (a) interfaces declared in non-test files with their
  method sets and (b) named types declared in non-test files with their method sets. The finish
  step does the `types.Implements` matching after loading, so no package needs another's types
  at pass time. Method sets are recorded as identities the finish step re-resolves against the
  loaded `types.Package`s rather than serialized signatures.
- `restrictions`: a package fact with the parsed declaration (axes, patterns). TS-P02 is computed
  forward in the pass from imported packages' facts — a per-package rule that needs no finish
  step.
- `closedworld`: no facts. `Requires: buildssa`, reads the restrict declaration by parsing the
  package doc comment through `internal/directive` (the same parse `restrictions` does; a shared
  helper in the internal library returns the parsed declaration or nothing).

### Directive grammar

`internal/directive` gains `ParseRestrict`/`FormatRestrict` following the `effects` precedent:
a comma-separated list of axes, each `closed-dispatch`, `no-reflect`, or
`imports(<path>[, <path>...])`; paths are module-relative, optionally ending `/...`; round-trip
canonical form sorts axes in that order and paths lexically. `Parse` validates intent-verb
arguments for `restrict` the way it validates pin-verb arguments today; other intent verbs stay
free text. `pins.Collect` is not extended — it is pin-scoped by design; the restrict declaration
is read from `file.Doc` on the file that carries it, and the package clause is the only node it
can bind to.

### Corpus harness for whole-program rules

`analysistest` cannot run a finish step, so each whole-program rule's corpus is a small module
under `internal/analyzers/<name>/testdata/module/` (own `go.mod`, two or more packages) run
through `driver.Check` with the analyzer alone. Expectations are `// want "regexp"` comments on
the line the finding lands, checked by a shared helper in `internal/driver/drivertest` that
parses them the way `analysistest` does. The meta-test's corpus-presence check accepts either
layout (`testdata/src/<ruleid>/` or `testdata/module/`) per rule, and the message-style replay
runs the module layout through the same helper so every emitted message is style-checked.

## Data Design

No persistent data. Facts are in-memory per run. The new source-level artifacts are the five
corpora, the two-package plugin-smoke fixture, and the ADR.

### Invariant Preservation

- Invariant 1 (position in module): the finish helper in the driver rejects diagnostics whose
  position is not in a loaded package's `Fset` file set before appending; enforced structurally
  at the one merge point, not per analyzer.
- Invariant 2 (fact ownership): the facts store already asserts on export of a foreign object;
  package facts are always on `pass.Pkg`. Nothing new to enforce.
- Invariant 3 (determinism): input order is the driver's; each finish function sorts its own
  output keys. Application-logic enforcement, so the double-run fixture test covers every
  whole-program rule.

### Illegal State Analysis

A finish function registered for an analyzer that is not in the registry cannot exist: the field
lives on `CustomRule`. A whole-program rule without a module corpus fails
`TestEveryRegisteredRuleHasCorpus`. A finish step that runs after a failed pass cannot happen:
`Check` returns on the first pass error before the finish loop.

## Security

No new inputs beyond Go source already loaded. The `imports(...)` patterns are compared as
cleaned path prefixes, never used as filesystem globs.

## PII

None. Findings carry file paths and identifiers, as today.

## Scale

The finish step is linear in the number of exported facts. `singleimpl`'s matching is
interfaces × types per module with `types.Implements`; tiger's own tree is the first benchmark and
the acceptance criterion is "no observable slowdown of `tiger check ./...`" — measure before and
after on the git-server mono-repo used in ENG-159 and record the numbers in the PR.

## Testing

Testing follows the `surface-testing` skill.

Key surfaces:
- **Per-analyzer corpus** via `analysistest` for `restrictions`, `closedworld` (helper packages
  alongside rule-id dirs, per the `ts-f02helper` precedent) and via the module harness for the
  three whole-program rules.
- **Driver**: `driver.Check` on a fixture module with a probe finisher, proving order of
  packages and facts handed to the finish step, panic containment (exit 2, no findings),
  position validation, and merged sorting. Extends `TestCheckPlumbsDependenciesFactsAndOrder`.
- **CLI**: `cli.Run` on `internal/cli/testdata/fixtures/` modules for acceptance criteria 1–5,
  including the `--show-facts` TS-P02 lines and the double-run determinism test.
- **Plugin**: `plugin_test.go` asserts `BuildAnalyzers()` returns no finish behavior and the
  per-package halves export facts; CI `plugin-smoke` runs the real binary on the two-package
  fixture (criterion 7).
- **Meta-tests**: registry coherence, corpus presence for both layouts, message style over both
  replay paths.
- **Fakes needed**: none. Fixture modules are real Go source.

Time and concurrency do not enter; no clock injection needed.

## Limitations & Future Work

- Whole-program rules are `tiger check`-only. If golangci-lint ever exposes a module-level hook,
  the finish contract is the thing to wire to it.
- `surfacediff` (TS-R02, TS-P03) and `surfaces` (TS-I0x) are separate tickets; `surfaces` first
  needs a directive form for surface declarations, which is a specification change.
- TS-K03 devirtualization is intra-function only. A receiver built in a helper and returned is a
  finding. If trial evidence shows that shape dominating, the fix is a facts-carried "returns a
  single concrete type" summary from `effects`, not an exemption.
- TS-P02 reports only module-internal bounds; a third-party import is the weakest on every axis
  by rule, which is honest and keeps the incentive pointing at TS-D01.

## Open Questions

- Whether tiger's own tree has TS-X01 findings once `singleimpl` registers (acceptance criterion
  8 says fix them). The count decides whether the message's "delete the interface" edit is the
  one people actually make; if the trial on ENG-159's mono-repo shows the rule dominated by
  plugin-shaped interfaces with one in-tree implementation and out-of-tree consumers, that is an
  ADR-0006 conversation, not a threshold.
