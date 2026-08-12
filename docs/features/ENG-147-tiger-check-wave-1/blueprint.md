# tiger check — Foundation and Wave-1 Analyzers Blueprint

Source specification: the Tiger Go Specification (145 rules, Parts I–V, IDs `TS-*`), maintained in
the design vault. This Blueprint covers the first buildable slice of the `tiger` tool; every rule ID
below refers to that document.

## Objective

Build the `tiger` analyzer binary — the machine that checks Tiger Go code against the rules no
off-the-shelf linter covers. Version 1 delivers the foundation (CLI, directive grammar, rule
registry, corpus conventions) plus the 20 wave-1 analyzers: every custom rule that is computable
from a single package's AST and types, with no SSA, no cross-package facts, and no pins.

The premise this serves: questions a reviewer normally answers by reading become questions a tool
answers by checking. Wave 1 is the computed-only start of that — analyzers that need no
declarations from the code they check, so they give value on day one of adoption.

## Mental Model

Tiger Go enforcement has two engines, and this project builds only one of them:

- **Auto rules** are enforced by off-the-shelf golangci-lint linters, configured by
  `config/golangci.yml` (Stage 0, already in this repo). Tiger never reimplements a rule an
  existing linter covers.
- **Custom rules** are enforced by analyzers this project writes. Each is a standard
  `golang.org/x/tools/go/analysis` pass — the same shape as `go vet`'s passes — so it runs
  identically under the `tiger` CLI, under golangci-lint (via module plugin), and under the test
  driver.

Three concepts organize everything:

1. **The rule registry is the single source of rule identity.** Every analyzer declares which
   `TS-*` rules it enforces and at what severity. The binary, the documentation, and the
   acceptance meta-tests are all derived from the registry, so an analyzer cannot exist half-way:
   registered means built, corpus-tested, and shipped.
2. **The corpus is the executable specification of a rule.** Each rule's failure-mode example
   (from its "Why" in the spec) must fire the diagnostic; the compliant rewrite must stay silent;
   known coverage gaps are present and marked. A rule without a corpus does not merge.
3. **Analyzers are driver-agnostic.** An analyzer never knows whether the tiger CLI, golangci-lint,
   or a test is running it, and never decides run-level policy (severity, exit codes, output
   shape). Policy lives above the analyzer interface.

## Core Design Principles

- **AI agents are the primary consumer of diagnostics.** Every finding must be self-correcting: it
  names the rule, states what fired, and names the compliant form — enough to fix the finding
  without reading the spec. Humans read the same output.
- **Deterministic, offline, no LLM.** Checking is a pure function of the source tree. No network,
  no model calls, no wall-clock influence on output.
- **Driver-agnostic analyzers, uniform policy.** Severity (blocking / advisory / reported) is
  defined once, per rule, in the registry — never decided inside an analyzer. The three levels
  follow the spec: blocking fails the run, advisory is printed and counted but does not fail,
  reported is annotation-only. Wave 1 uses blocking and advisory; the reported level exists in the
  model but has no wave-1 rules.
- **No silent scope cuts.** Where an analyzer covers less than its spec rule claims, the gap is a
  marked known-miss case in its corpus — visible, not implied.
- **Tiger extends the linter ecosystem, it does not compete with it.** Auto rules stay in
  golangci-lint; the module plugin merges both engines into one pass for consumers.

## Correctness Constraints

### State Invariants

1. **Every diagnostic names a rule ID that exists in the registry.** No orphan diagnostics.
   Violation is caught by a meta-test that walks every registered analyzer's corpus output.
2. **Directive text round-trips through the grammar package.** Parsing a printed directive yields
   the original structure (`Parse(Format(d)) == d`). This is the wave-1 form of the spec's output
   contract ("report and pin are one format"); when the effects wave starts printing computed
   facts, the same package guarantees report output is byte-identical to pin syntax. Enforced by a
   round-trip property test in the grammar package.
3. **Every registered analyzer has an acceptance corpus** containing, per rule it enforces: the
   failure-mode case (fires), the compliant rewrite (silent), and any known misses (marked).
   Enforced mechanically by a meta-test derived from the registry — not by review discipline.

### Behavioral Constraints

4. **Determinism.** The same source tree produces byte-identical output across runs and machines.
   Findings are ordered by position; map iteration order, timestamps, and absolute paths never
   reach the output. Verified in CI by running the check twice and diffing (the spec's TS-T11
   applied to tiger itself).
5. **Never report clean when a pass did not run.** If any analyzer fails on any package (panic,
   load error, type error), the run exits with an operational-failure code distinct from "findings
   found". Tiger never silently degrades to a smaller rule set. Exit codes: 0 clean, 1 findings,
   2 operational failure.
6. **No silent scope cuts** (constraint form of the design principle): a coverage gap without a
   marked known-miss corpus case is a defect.
7. **A false positive on a blocking rule is a bug in the analyzer, never resolved by adding a
   suppression directive.** The spec calls forced directives "how a dialect degrades into a style
   guide with extra steps." Process enforcement: any `//tiger:` or `//nolint` suppression of a
   tiger finding in this repo's own code requires a tracked bug against the analyzer. This is a
   review-level constraint; it cannot be fully mechanized.

## Acceptance Criteria

- `tiger check ./...` runs against this repository and against fixture modules, producing
  deterministic, position-sorted findings and the exit codes above.
- Every wave-1 analyzer passes its `analysistest` corpus: failure-mode fires, compliant rewrite
  silent, known misses marked. The corpus meta-test proves no registered analyzer lacks one.
- The directive grammar package round-trip property test passes for every directive form in the
  wave-1 vocabulary.
- A fixture module containing a package that fails to load exits 2, not 0 and not 1.
- CI runs the suite twice and diffs the output; any diff fails the build.
- The tiger repository dogfoods itself: CI runs Stage 0 golangci-lint plus `tiger check` over the
  tree, green.
- The golangci-lint module plugin registers all wave-1 analyzers and passes a smoke test (one
  known finding surfaces through a `golangci-lint` run).

Success is proven entirely by code designed to evoke a specific response — corpus packages at the
analyzer surface, fixture modules at the CLI surface. There is no post-ship metrics section: the
designed-response fixtures are the measure, per the surface-testing approach.

## Scope

### In Scope

**Foundation**

- The `tiger` CLI: `tiger check ./...`, built on subcommand dispatch so later commands
  (`pin`) slot in without breaking the interface. `main()` is a thin wrapper over a testable
  `Run`-style entry point (surface-testing).
- The directive grammar package: parses and prints the `//tiger:<verb>` namespace. Owns the verb
  vocabulary — an unknown verb is an error, not a silently meaningless comment. Wave-1 verbs:
  the escape hatches (`bounded`, `batched`, rule-deviation form) and validation that intent/pin
  verbs it does not yet understand are at least well-formed.
- The rule registry: rule ID → enforcing analyzer, severity, and spec cross-reference. Drives the
  binary's analyzer set, the corpus meta-test, and the severity behavior of the run.
- Corpus conventions shared by all analyzers (failure-mode / compliant / known-miss case classes).
- The golangci-lint module plugin (end of v1, after the CLI works).

**Wave-1 analyzers (20)** — inclusion criterion: computable from a single package's AST and
types; no SSA, no cross-package facts, no pins; exact by construction or with known-miss corpus
cases. The binding constraint is correctness constraint 7 (false positives), not effort.

| Analyzer | Rules | Severity |
| --- | --- | --- |
| `nogoto` | TS-S09 | blocking |
| `paniccheck` | TS-S18 | blocking |
| `boundedloop` | TS-S02, TS-S03 | blocking |
| `compoundcond` | TS-S06, TS-S07, TS-S08 (default arm) | blocking |
| `nogoroutine` | TS-C02, TS-C09 | blocking |
| `selectctx` | TS-C05 | blocking |
| `chandecl` | TS-C12 | blocking |
| `errignore` | TS-E02 | blocking |
| `returnarity` | TS-E06 | blocking |
| `directives` | TS-L09 | blocking |
| `skipcheck` | TS-D07 | blocking |
| `tablename` | TS-T10 | blocking |
| `testdoc` | TS-T06 | blocking |
| `derivation` | TS-S22 | blocking |
| `limitrelate` | TS-S21 | blocking |
| `sametypeparams` | TS-N07, TS-N08 | blocking |
| `namedeny` | TS-N12, TS-N13 | blocking |
| `namepairs` | TS-N15 | blocking |
| `participle` | TS-N14 | blocking |
| `deferdistance` | TS-L10 | blocking (defer in loop), advisory (distance) |

### Out of Scope / Non-Goals

- **The `tiger pin` subcommand.** Wave-1 analyzers produce no pinnable computed facts
  (effects, frames, and variants arrive with the SSA waves). The grammar package ships in full so
  the contract exists before the command that uses it; `pin` lands with the effects wave.
- **SSA-based analyzers**: `norecursion`, `maporder`, `poolzero`, the `effects` engine, `frames`,
  `variant`, `contracts`.
- **Cross-package-fact analyzers**: `invariantrefs`, `invariantnegative`, `restrictions`,
  `surfacediff`, `closedworld`, `singleimpl`.
- **Tune-heavy / heuristic analyzers (wave 1.5)**: `noabbrev` ("the dictionary is the work"),
  `restatement`, `declusedistance`, `assertdensity`, `invariantsymmetry`, `paradigm`,
  `queuebound`, `declorder`, `unitsuffix`, `ioinloop`, `outptr`, `passthrough`, `corpuscheck`,
  `fuzzcoverage`, `adapters`, `ownership`, `escapecheck`, `canonical`, `wiretypes`, `hotpath`,
  `nofloat`, `quantitycast`, `domaintypes`. Deferred until corpus conventions are proven on the
  exact tier.
- **Reimplementing any auto rule** golangci-lint already enforces.
- **Auto-fix, editor/LSP integration, dashboards, the TS-D06 ratchet tooling, the refactor
  prover.**
- **Verifying tracker IDs against a live tracker** (`skipcheck` checks directive shape and the
  presence of an ID token; the live-tracker check is a CI concern for adopting projects, per
  TS-D05's runtime half).

## Dependencies and Constraints

- Go 1.26+; `golang.org/x/tools` (analysis framework, `analysistest`), pinned exactly and bumped
  deliberately. golangci-lint v2 for the auto rules and the plugin mechanism
  (`plugin-module-register`). testify for tests. Nothing else.
- The `assert` package and the invariant vocabulary pattern (`examples/ledger`) already exist in
  this repo and are unchanged by this work.
- Implementation is AI-driven; there are no effort estimates. The binding constraint is functional
  acceptance per the acceptance contract above.

User stories are deliberately omitted: this is developer-infrastructure work with a single
consumer type, and the correctness constraints and acceptance criteria are already at story
granularity.

---

## Functional

`tiger check [packages]` loads the named packages, runs every registered wave-1 analyzer, and
prints findings ordered by file position.

**Diagnostic contract.** Every finding carries: the rule ID (`TS-*`), the position, what fired,
and the compliant form — self-correcting for an AI agent, readable by a human. One consistent
message shape across all analyzers; exact wording per analyzer is the implementor's, but the
corpus asserts the shape (rule ID present, compliant form named).

**Severity contract.** The registry defines each rule's severity. Blocking findings fail the run
(exit 1). Advisory findings print, marked as advisory, and do not affect the exit code. Reported
exists in the model for later waves. A rule with split severity (TS-L10) registers each half at
its own level.

**Escape hatches.** `//tiger:bounded <reason>` and `//tiger:batched <reason>` suppress their rule
at one site; the `directives` analyzer fails any escape hatch missing a reason (TS-L09) and any
`//tiger:` line whose verb is not in the vocabulary.

**Exit codes.** 0 clean, 1 at least one blocking finding, 2 operational failure (any package
failed to load or any analyzer failed to run — partial results are never presented as a clean or
complete run).

## Architecture

Components and their contracts — internal shapes are the implementor's to design:

- **Analyzers** (one package per analyzer, each with its own corpus). Pure
  `go/analysis` passes over a single package's AST and types. They receive parsed directives from
  the grammar package, emit diagnostics tagged with their rule ID, and contain no severity, exit,
  or output-formatting logic. An analyzer must behave identically under the tiger CLI,
  golangci-lint, and `analysistest` — this opacity is what makes the plugin a shim rather than a
  port.
- **Directive grammar package** (internal library boundary with its own surface tests).
  Responsibility: the `//tiger:` namespace — recognizing directives in comment groups, validating
  verbs against the closed vocabulary, extracting arguments and positions, and printing directives
  canonically. The round-trip invariant (constraint 2) is this package's contract. Per-verb
  argument grammars beyond wave-1's needs (effect lattices, frame sets) are explicitly not its
  wave-1 job, but the package is the place they will live.
- **Rule registry** (internal). Responsibility: the closed set of (rule ID, analyzer, severity,
  spec reference). Consumed by the CLI driver to assemble the run and decide exit behavior, and by
  the meta-tests to enforce constraints 1 and 3. Illegal states unrepresentable: the binary's
  analyzer list is *derived from* the registry, so "shipped but unregistered" cannot be expressed.
- **CLI driver** (`cmd/tiger` delegating to a testable run function). Responsibility: subcommand
  dispatch, package loading, running the registered analyzers, ordering findings, applying
  severity, exit codes. All run-level policy lives here.
- **golangci-lint plugin** (thin registration shim in the same module). Responsibility: expose the
  same registered analyzers to golangci-lint. Contains no logic — if it needs any, the
  driver-agnosticism principle has been violated somewhere else.

Configuration (dictionaries for `namedeny`/`namepairs`/`participle`, the supervisor allowlist for
`nogoroutine`) rides the standard per-analyzer flag mechanism, which both the CLI and
golangci-lint's settings can drive; no bespoke config file in v1.

### Invariant Preservation

- Constraint 1 (no orphan diagnostics): the registry is the only path to being in the binary, and
  the meta-test cross-checks every corpus diagnostic's rule ID against it. Adding an analyzer
  without registering it is structurally impossible; registering without a corpus fails the
  meta-test.
- Constraint 2 (round-trip): single grammar package owns parse and print; no analyzer or driver
  formats directive text by hand. Enforced by the property test plus review of any second
  formatting site as a defect.
- Constraint 4 (determinism): findings are sorted by position in the driver before output; no
  analyzer output reaches the user unsorted. The double-run diff in CI is the backstop that
  catches sources the design did not anticipate.
- Constraint 5 (no clean-on-partial): the driver treats load and analyzer errors as terminal for
  the run's exit code, never as skippable. The fixture test binds the behavior.

## Data Design

No persistent data. The only durable artifacts are source-level: the registry (in code), the
corpora (testdata packages), and the dictionaries/tables for the naming analyzers (committed
files, since TS-N15's table and TS-N12's token list are project-owned vocabulary).

## Security

Tiger parses and type-checks untrusted Go source but never executes it, and makes no network
calls. The plugin runs inside golangci-lint's process with the same property. Threat surface is
that of any static analyzer: malformed input must produce exit 2, never a hang (bounded work) or
a crash presented as a clean run.

## PII

None. Tiger processes source code only; findings contain file positions and identifiers, no
personal data.

## Testing

Testing follows the `surface-testing` skill. All tests exercise code designed to evoke a specific
response.

Key surfaces:

- **Per-analyzer**: `analysistest` against the analyzer's corpus — the driver API is the
  analyzer's real consumer. Corpus case classes per rule: failure-mode (fires), compliant rewrite
  (silent), known-miss (silent, marked as a documented gap).
- **CLI**: the testable run entry point called with fixture modules; asserts exit codes and output
  bytes (determinism, ordering, severity behavior, partial-failure exit).
- **Directive grammar package**: an internal library boundary tested through its exported API,
  including the round-trip property test.
- **Meta-tests**: derived from the registry — every registered analyzer has a corpus with the
  required case classes; every diagnostic rule ID resolves.
- **Plugin**: one smoke test — a known finding surfaces through a golangci-lint run.
- Fakes needed: none. No external services, no async behavior, no time dependence (banned by the
  determinism constraint), so no fakes, clocks, or observability APIs are required.

Tests live in `package xxx_test`, use testify, and follow the repo's testing conventions.

## Limitations & Future Work

- **Wave 1.5**: the tune-heavy exact-tier analyzers (`assertdensity`, `invariantsymmetry`,
  `declorder`, `unitsuffix`, `noabbrev`, `restatement`, `declusedistance`, `paradigm`,
  `queuebound`, `ioinloop`, `outptr`, `passthrough`), riding the proven corpus conventions.
- **Chain 1 (ownership)**: `ownership`, `escapecheck`, `chandecl` extensions — first SSA wave.
- **Effects wave**: the `effects` engine (inference-first), `frames`, `variant`, `contracts`; the
  `tiger pin` subcommand ships here, carrying its deferred constraint — pin writing touches
  nothing but the pin line and is idempotent.
- **The reported severity level** activates when unpinned-fact reporting exists.
- **Ratchet tooling** (TS-D06 budgets), `surfacediff`, and the refactor prover are their own
  campaigns.

## Open Questions

- Whether `chandecl`'s "channel types used across goroutines" can be decided exactly from
  single-package AST or needs a documented known-miss for channels passed through function values.
  Resolved during implementation by the corpus — either answer is acceptable, silence about it is
  not.
- Where the `namedeny` / `namepairs` committed dictionaries live and how adopting projects extend
  them (per-repo allowlist with reasons, per TS-N12). Shape is settled; ownership story lands with
  the plugin documentation.
