# tiger check — SSA Wave Blueprint

Source specification: the Tiger Go Specification (`docs/Tiger Specification.md`, rules `TS-*`).
Builds on the wave-1 foundation (`docs/features/ENG-147-tiger-check-wave-1/blueprint.md`): the CLI,
directive grammar, rule registry, corpus conventions, and driver all exist; this wave extends them.

## Objective

Build the analyzers that need SSA — a compiler-style representation of what code actually does, not
just how it is written. Wave 1 deliberately excluded SSA to keep every analyzer single-package and
fast; this wave introduces it, delivering the seven deferred analyzers: `norecursion`, `maporder`,
`poolzero`, and the effects engine (`effects`, `frames`, `variant`, `contracts`).

The wave also activates two dormant pieces of the wave-1 design: the **reported** severity level
(computed facts, printed on request) and the pin verbs the directive grammar already parses
(`effects`, `frame`, `requires`, `ensures`, `variant`) — this wave makes them enforceable.

`tiger pin` is **not** this wave: it is ENG-151, scheduled immediately after this wave merges. The
binding constraint that deferral imposes here: every computed fact this wave prints must be in
exact pin syntax — `directive.Format` output that `directive.Parse` round-trips — so ENG-151 can
freeze a printed fact into a pin by pasting it.

## Mental Model

Wave 1 checked how code is written; this wave checks what code does. Three concepts organize it:

1. **The fact lifecycle: computed → reported → pinned.** The analyzer computes a fact (an effect
   set, a frame, a variant) for every function with no annotation required. Computed facts are
   visible on request (`--show-facts`, the reported severity level activating). A pin freezes a
   computed fact into a blocking contract: once pinned, the computed fact must match the pin
   exactly — in both directions. Absence of a pin never means the fact is absent (CONTEXT.md).
2. **Effects are transitive by construction.** A function's effect set summarizes everything
   reachable beneath it: its own instructions, its callees, their callees. This is what turns a
   syntactic ban ("no `time.Sleep`") into a behavioral guarantee that cannot be laundered through
   a helper three frames down. Transitivity crosses package boundaries through the `go/analysis`
   facts mechanism (same module) and a curated standard-library effects table (stdlib).
3. **Exactness is still the bar.** Correctness constraint 7 carries over unchanged: a false
   positive on a blocking rule is a bug in the analyzer, and there is still no dismissal
   directive. Where SSA analysis cannot decide, the undecidable case is a marked known-miss,
   never a maybe-finding. Two spec enforcement texts are deliberately amended to hold this bar
   (see `norecursion` and `maporder` in Functional).

## Core Design Principles

Wave-1 principles carry over unchanged (AI agents are the primary consumer; deterministic, offline,
no LLM; driver-agnostic analyzers; no silent scope cuts; escape hatches must earn their existence;
escapes are never silent). This wave adds:

- **Report output is pin syntax.** A computed fact is printed exactly as its pin would be written —
  `directive.Format` of the fact's directive form, byte-identical, round-trippable. There is one
  formatting site (the grammar package) and one syntax; freezing a fact (ENG-151) is a paste.
- **Pins are exact in both directions.** On a pinned function, an undeclared computed effect fails
  and so does a declared-but-absent effect. This is what stops a pin drifting into a defensive
  superset that means nothing (TS-F01).
- **Sparse pins, dense enforcement.** A pin bounds the entire subtree beneath it (TS-F02): pinning
  a package's exported surface constrains every unexported helper under it, across package
  boundaries via facts. Where a pinned function calls another pinned function, the callee's pin is
  checked from signatures alone — checking stays modular exactly where pins are dense.
- **The reported level is quiet by default.** Reported findings print only under `--show-facts`
  and never affect the exit code. A default run's output is findings, not inventory.
- **Undecidability resolves to a known-miss, never a finding.** SSA analyses (points-to, call
  graphs, variant synthesis) have hard limits. Every limit is a marked corpus case; none is a
  diagnostic.

## Correctness Constraints

### State Invariants

Wave-1 invariants 1–3 (registry-resolved diagnostics, directive round-trip, corpus per rule) carry
over and extend to the new analyzers automatically — the meta-tests are registry-derived. New:

1. **Fact output round-trips through the grammar package.** Every fact printed by `--show-facts`
   is the `directive.Format` of a directive the grammar parses back to the same structure. This
   discharges wave-1's forward reference ("when the effects wave starts printing computed facts,
   the same package guarantees report output is byte-identical to pin syntax"). Violation is
   caught by a round-trip test over the fixture module's printed facts.
2. **The effect lattice is closed.** `alloc`, `io(q)`, `block`, `panic`, `rand`, `time`,
   `mutate(x)`, `spawn`; purity is the empty set, spelled `none` in pin syntax. `io` qualifiers
   this wave are the built-in tier only: `disk`, `net`, `exec`, `env`. A pin naming anything
   outside the closed set is a blocking parse error (TS-L09) — a misspelled qualifier is an
   error, never a silently meaningless string.
3. **A pin may only appear on an exported function or method** (TS-F01). A pin on an unexported
   function is a blocking finding.

### Behavioral Constraints

Wave-1 constraints 4–7 (determinism, never clean-on-partial, no silent scope cuts, blocking false
positive is a bug) carry over. New:

4. **Reported findings never affect the exit code**, printed or not. Blocking and advisory
   semantics are unchanged.
5. **Fact computation is deterministic.** Package visit order is a stable topological order;
   fact content never depends on map iteration or scheduling. The CI double-run diff extends to
   a `--show-facts` run.
6. **Pin enforcement is bidirectional.** Undeclared computed effect: blocking. Declared but
   absent effect: blocking. Same for frames.
7. **Every blocking finding still names an in-dialect rewrite.** No new analyzer creates a
   finding whose only exit is a directive: `variant`'s finding names the explicit-counter-cap
   rewrite (itself a synthesizable linear variant), `maporder`'s names sorted-keys iteration,
   `norecursion`'s names the explicit stack/queue form. The spec's TS-V01 note that an
   out-of-language ranking "needs a deviation with a reason" is deliberately not implemented —
   the deviation directive still fails ADR-0003's admission test, and the counter-cap rewrite is
   always available in-dialect.

## Acceptance Criteria

- Every new analyzer passes its `analysistest` corpus: failure-mode fires, compliant rewrite
  silent, known misses marked. The registry-derived meta-tests cover the new analyzers with no
  meta-test changes.
- The driver runs `Requires`-dependent analyzers: `buildssa` results are plumbed, and the wave-1
  assertions against `Requires` and `FactTypes` are lifted.
- A multi-package fixture module proves facts propagation: a pinned function whose effect
  violation is introduced in a *different package* of the fixture produces the blocking finding
  at the pin, naming the introducing call (TS-F02).
- `tiger check --show-facts` against a fixture module prints computed effect sets, frames, and
  synthesized variants for exported functions in pin syntax; a test parses every printed fact
  back through the grammar (invariant 1). Without the flag, output is byte-identical to a wave-1
  run over the same tree.
- Exit codes are unchanged: reported findings never alter them; a fixture with only unpinned
  facts and no violations exits 0.
- The CI determinism check (run twice, diff) passes with `--show-facts` on.
- The tiger repository dogfoods itself: `tiger check` over this tree runs the new analyzers
  green — including fixing any of tiger's own code the new analyzers legitimately flag.
- The golangci-lint plugin registers the new analyzers unchanged and the smoke test still passes.

Success is proven by designed-response fixtures, per surface-testing; there is no post-ship
metrics section.

## Scope

### In Scope

**Driver evolution**

- Lift the wave-1 `Requires`/`FactTypes` assertions — deliberately, per the ticket.
- Resolve analyzer dependencies (`buildssa`) and plumb their results.
- Facts propagation: visit the module's packages in stable topological order, carrying exported
  facts from dependency to dependent. Facts are in-memory and module-local.
- The `--show-facts` flag on `tiger check`.

**Grammar extension**

- Per-verb argument grammars for the pin verbs the vocabulary already names: the effect lattice
  syntax (`mutate(r.log, r.checkpoint), io(disk)`, `none`), frame location lists, variant
  expressions, and the contracts predicate language (nil-ness, integer ranges, length relations,
  invariant IDs). The round-trip property test extends to every new form. Malformed arguments are
  blocking TS-L09 findings through the existing `directives` analyzer.

**Analyzers (7)**

| Analyzer | Rules | Severity |
| --- | --- | --- |
| `norecursion` | TS-S01 | blocking |
| `maporder` | TS-T02 | blocking |
| `poolzero` | TS-M05 | blocking |
| `effects` | TS-F01, TS-F02 | blocking (pin violations), reported (computed facts) |
| `frames` | TS-F07 | blocking (pinned-frame violations), reported (computed frames) |
| `variant` | TS-V01 | blocking (no variant, or pinned variant fails), reported (synthesized) |
| `contracts` | TS-V03 | blocking (proven violation only) |

### Out of Scope / Non-Goals

- **`tiger pin`** — ENG-151, immediately after this wave. This wave's obligation to it: printed
  facts are freeze-ready pin syntax.
- **Effects rules beyond the core**: TS-F03 (`hot`, needs `hotpath`, wave 1.5), TS-F04 (assertion
  purity), TS-F05 (critical sections), TS-F08 (`spawn` supervisor binding), TS-V02 (totality).
  Each rides on this wave's computed effect sets and lands with a later wave; the `spawn` effect
  itself is still computed and reported now.
- **The declared io-qualifier tier** (`io(database)` anchored to surface declarations) — needs
  the `surfaces` analyzer (cross-package wave). Built-in tier only this wave.
- **Cross-package recursion detection** — `norecursion` is intra-package; cross-package cycles
  are a marked known-miss. The facts plumbing this wave builds makes this a natural follow-up.
- **Per-dependency fact caching / third-party analysis.** Calls into modules outside this one
  (not stdlib, not the analyzed module) are assumed effect-free — a marked known-miss.
- **The deviation directive and ratchet** — unchanged from wave 1; ADR-0003 stands.
- **Auto-fix, LSP, dashboards** — unchanged.

## Dependencies and Constraints

- Everything wave 1 pinned: Go 1.26+, `golang.org/x/tools` (now also `go/ssa`,
  `go/analysis/passes/buildssa`), golangci-lint v2, testify. Nothing new.
- ENG-151 (`tiger pin`) depends on this wave's fact-output contract; do not break pin-syntax
  compatibility after merge without coordinating with it.
- Implementation is AI-driven; the binding constraint is the acceptance contract above.

User stories are deliberately omitted, as in wave 1: developer infrastructure, one consumer type,
constraints already at story granularity.

---

## Functional

### `norecursion` — TS-S01, blocking

Fires on any cycle in the package's static call graph: direct function calls and method calls on
concrete types, including mutual recursion and calls through function literals with a statically
known callee. The finding names the cycle's members and the compliant form — an explicit
stack or queue with a bound.

**Deliberate deviation from spec text.** The spec says "CHA callgraph." CHA (class hierarchy
analysis) resolves an interface call to every implementing type, so it reports cycles that cannot
occur at runtime — blocking false positives, which constraint 7 defines as bugs. This wave uses
the static call graph instead: every reported cycle is real. Known-misses: recursion through
interface method calls, through function values whose target is not statically known, and cycles
spanning packages. Flagged for spec amendment alongside the wave-1 TS-S02 amendment.

### `maporder` — TS-T02, blocking

Fires on every `range` over a map unless the loop body is one of a closed allowlist of provably
order-insensitive shapes:

- writes into another map (including set-building `m[k] = struct{}{}` / `= true`)
- commutative integer accumulation (`+=`, `-=`, counters, integer min/max)
- `delete` from a map
- pure per-element checks that only short-circuit a boolean

Everything else — appends, writer calls, string building, any call whose argument derives from
the iteration — is a finding naming the compliant form: collect the keys, sort
(`slices.Sorted(maps.Keys(m))`), iterate in sorted order.

**Deliberate deviation from spec text.** The spec's enforcement ("range over a map whose body
appends or writes. Heuristic") traces map order to a sink, which is undecidable in general and
would make a blocking rule fire on maybes. Inverting the rule — ban the range unless the body is
provably order-safe — is exact: every finding is a real order dependence or a loop the closed
allowlist cannot prove safe, and the rewrite is always available. This trades a small amount of
sorted-iteration busywork for zero false-positive risk on a blocking rule. Flagged for spec
amendment. The allowlist is closed and versioned with the analyzer; extending it is an analyzer
change with corpus cases, not configuration.

### `poolzero` — TS-M05, blocking

Two checks: a type used as a `sync.Pool` element implements `Reset`, and every `Pool.Put(x)` is
dominated by a reset of `x` — a `Reset` call or a zeroing store — on every path reaching the
`Put`. SSA dominance makes "preceded by" exact. Known-misses: pools or pooled values passed
across function boundaries, aliased pooled values.

### `effects` — TS-F01 + TS-F02

Computes every function's effect set over the closed lattice. Sources, in composition order:

- **Own instructions**: `go` → `spawn`; allocation instructions → `alloc`; channel operations,
  mutex acquisition, `select` without default → `block`; `panic` → `panic`; stores through
  parameters or receiver → `mutate(x)` where `x` is the parameter-rooted path (`r.log`).
- **The stdlib effects table**: a curated, versioned mapping from standard-library functions to
  effects — `os.(*File).Write` → `io(disk)`, `net.Dial` → `io(net)`, `time.Now` → `time`,
  `os/exec` → `io(exec)`, `os.Getenv` → `io(env)`, `math/rand` → `rand`, and so on. The table is
  a committed artifact with its own tests; an unlisted stdlib call contributes nothing and the
  table's coverage gaps are the analyzer's known-misses.
- **Transitive closure** over static calls: same-package directly, same-module through exported
  facts, third-party assumed effect-free (marked known-miss).
- **Pins as modular summaries**: a call to a pinned function contributes the pinned set, from the
  signature alone (TS-F02's modularity).

Enforcement is on pinned functions only, bidirectional (constraint 6). A widening introduced
anywhere in a pinned subtree fails **at the pin**, with the introducing call named (TS-F02) — the
finding tells the agent which call to remove or which pin to update, in pin syntax. A pin on an
unexported function is a blocking finding (invariant 3).

Unpinned functions: the computed set is exported as a fact and printed for exported functions
under `--show-facts` as a reported finding, in pin syntax.

### `frames` — TS-F07

Computes every function's frame: the set of locations reachable from its parameters and receiver
that it writes. Enforcement on pinned frames only: a write outside the pinned frame is blocking,
bidirectional (a pinned location never written is also a finding). Points-to analysis runs over
the restricted fragment the dialect's other rules produce; aliasing the fragment cannot decide is
a known-miss, never a finding. Unpinned frames: facts + `--show-facts`, same as effects.

### `variant` — TS-V01

For every condition-controlled loop that is not structurally terminating (`range` over data,
fixed-count `for i := 0; i < n; i++`) and not the TS-S03 event-loop shape:

- **Synthesis**: attempt a linear integer expression over locals (`len(pending)`, `high - low`)
  required to strictly decrease on every back edge and be bounded below. Success proves the loop;
  the synthesized variant is a reported fact (`--show-facts`, pin syntax, pinnable by ENG-151).
- **Pinned variant**: verified the same way; failure to decrease, or an expression outside the
  predicate language, is blocking.
- **Neither**: blocking. The finding names the always-available rewrite — an explicit iteration
  cap with an assert on exhaustion, whose counter is itself a synthesizable linear variant — so
  the exit is a code change, never a directive (constraint 7's shape, per ADR-0003).

### `contracts` — TS-V03

Parses `//tiger:requires` and `//tiger:ensures` predicates in the restricted language (nil-ness,
integer ranges, length relations, invariant IDs). At every call site of a function with a
`requires`, checks the condition is established by a dominating assertion, a type, or a preceding
check; in the body of a function with an `ensures`, checks the postcondition where provable.

Only a **proven violation** is blocking — a call site where the analyzer can show the
precondition false, a return where the postcondition provably fails. Unproven obligations are
silent: they degrade to the callee's runtime assertion (TS-A02), which is the spec's stated
default — the analyzer's incompleteness never blocks a build. Partial by nature; the corpus marks
what the predicate language cannot see.

### Severity and the registry

New registry entries per the scope table, following the wave-1 split-severity pattern
(`TS-F01` blocking / `TS-F01-facts` reported, and likewise for frames and variant). The
**reported** level activates: reported findings are collected like all findings, printed only
when `--show-facts` is set, never counted toward the exit code. Blocking and advisory behavior
is byte-identical to wave 1 when the flag is absent.

### Driver

- The wave-1 assertions that `Requires` and `FactTypes` are empty are lifted.
- Analyzer dependency graphs are resolved and results plumbed (`buildssa` computed once per
  package, shared by every SSA analyzer).
- Packages are visited in a stable topological order; analyzers' exported facts flow from
  dependency to dependent, in memory, within the analyzed module. Load mode extends as needed
  (`NeedDeps`); the exact mode set is the implementor's.
- Everything else — severity in the CLI, exit codes, sorting, generated-file policy, panic
  containment — is unchanged (ADR-0002 stands).

## Architecture

New components and contracts; internal shapes are the implementor's:

- **Seven analyzer packages** (`internal/analyzers/<name>`, each with its own corpus), pure
  `go/analysis` passes declaring `Requires: buildssa` (except `maporder`, which may be
  AST-decidable — implementor's call) and, for `effects`/`frames`, exported fact types.
  Driver-agnostic as ever: identical under the tiger CLI, golangci-lint (whose runner handles
  `Requires` and facts natively), and `analysistest`.
- **A shared SSA/effects internal library** (`internal/analyzers/internal/...`, mirroring the
  `words` precedent): the effect lattice types, the stdlib effects table, static-call-graph
  walking, dominance helpers. Contract: the lattice is the closed set of invariant 2; the table
  is data with tests, not code with opinions.
- **The grammar package grows per-verb argument grammars.** Contract: every pin verb's arguments
  parse to structure and format canonically; the round-trip invariant extends to every new form;
  parse errors carry enough to name the offending token in a TS-L09 finding. The structured
  forms are what the analyzers compare against computed facts and what `--show-facts` formats.
  This is the seam ENG-151 builds on.
- **The driver grows dependency resolution and facts transport** (contract above). The CLI grows
  one flag. The registry grows the new entries and nothing structural.

### Invariant Preservation

- Invariant 1 (fact round-trip): the grammar package remains the single formatting site;
  `--show-facts` output is produced by `directive.Format` calls, nothing hand-built. Enforced by
  the extended property test plus the fixture parse-back test.
- Invariant 2 (closed lattice): the lattice is a Go type in the shared library; a pin argument
  outside it fails parsing (structural), not comparison (logical).
- Invariant 3 (pins on exported only): checked by `effects`/`frames` at the declaration site.
- Constraint 5 (fact determinism): topological order with a stable tie-break; the double-run
  diff backstops what the design did not anticipate, now including facts output.
- Constraint 6 (bidirectional pins): both directions are corpus cases for every pin-bearing
  analyzer — the meta-test's failure-mode requirement covers firing, and the corpus convention
  adds the superset-pin case explicitly.
- Constraint 7 (in-dialect rewrite named): the corpus asserts message shape as in wave 1; each
  blocking rule's failure-mode message names its rewrite.

## Data Design

No persistent data, as in wave 1. New durable source-level artifacts: the stdlib effects table
(committed, tested) and the `maporder` allowlist (code with corpus cases, not configuration).
Facts are in-memory per run — never serialized to disk this wave.

## Security

Unchanged from wave 1: tiger parses and type-checks untrusted Go source, never executes it, no
network. SSA construction is still static analysis; malformed input must produce exit 2, never a
hang or a crash presented as clean.

## PII

None. Source code in, positions and identifiers out.

## Testing

Testing follows the `surface-testing` skill; all wave-1 surfaces carry over.

Key surfaces:

- **Per-analyzer**: `analysistest` against each corpus — it resolves `Requires` and facts
  natively, so SSA analyzers test exactly like wave-1 analyzers. Corpus case classes unchanged
  (failure-mode / compliant / known-miss), plus for pin-bearing analyzers: the superset-pin
  failure case (declared-but-absent fires) and a facts case (cross-package propagation within
  the corpus module).
- **CLI**: fixture modules through the run entry point — the multi-package facts fixture
  (violation introduced across a package boundary, finding at the pin), the `--show-facts`
  golden output plus parse-back test, the flag-absent byte-identity check, determinism
  double-run with the flag on.
- **Grammar package**: round-trip property test extended over every pin-verb argument grammar.
- **Stdlib effects table**: its own tests in the shared library.
- **Meta-tests**: unchanged — registry-derived, they cover the new analyzers automatically.
- **Plugin**: existing smoke test; one added case with an SSA analyzer finding.
- Fakes needed: none, as before.

## Limitations & Future Work

- **Cross-package `norecursion`** rides the facts plumbing this wave builds.
- **The declared io tier** (`io(database)`) lands with `surfaces` (TS-I01, cross-package wave).
- **TS-F04, TS-F05, TS-F08, TS-V02, TS-F03** — deferred effects-family rules; each consumes this
  wave's computed sets.
- **Third-party effect analysis / fact caching** — this wave assumes non-module, non-stdlib
  calls are effect-free (marked known-miss).
- **ENG-151 (`tiger pin`)** freezes what `--show-facts` prints; the pin-writing constraints
  (touches nothing but the pin line, idempotent) are recorded in the wave-1 blueprint and bind
  that ticket, not this one.
- **Spec amendments to file**: TS-S01 enforcement (static call graph, not CHA), TS-T02
  enforcement (closed-allowlist ban, not sink tracing), TS-V01 enforcement note (counter-cap
  rewrite replaces the deviation-directive path).

## Open Questions

- Whether `maporder`'s allowlist needs a "pure per-element check" arm at all, or whether the
  first corpus pass shows real code never needs it. Resolved by the corpus — either answer is
  fine, silence is not.
- Whether `frames`' points-to fragment can reuse `effects`' `mutate(x)` path computation or
  needs its own walk. Implementor's call; the shared library is the place either way.
- How large the initial stdlib effects table must be for the dogfood run to pass (tiger's own
  tree uses `os`, `fmt`, `go/*`). The dogfood criterion bounds it from below; growth after that
  is demand-driven.
