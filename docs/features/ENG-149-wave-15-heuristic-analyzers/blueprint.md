# Wave 1.5: Heuristic Analyzers Blueprint

## Objective

Ship the first tune-heavy tier of tiger: the `ioinloop` analyzer that completes the
`//tiger:batched` escape hatch, the wave-1 rule tuning both real-codebase trials proved
necessary (TS-S02 bound grammar, TS-S03/TS-C05 shutdown shapes, TS-S08 open enums), two cheap
new advisory analyzers (`declorder`, `declusedistance`), and the assert-adoption playbook that
gates the density rules. The wave-1 corpus conventions (failure / compliant / known-miss files
per rule, enforced by meta-test) carry over unchanged.

Evidence base: the querator trial ([ENG-148 report](../ENG-148-trial-tiger-on-a-real-codebase/trial-report.md))
and the git-server trial ([ENG-159 report](../ENG-159-trial-tiger-on-git-server-mono-repo/trial-report.md)).
Between them: TS-S02 found 5 of the 10 real bugs (including a remotely triggerable CPU-spin
DoS) while 55 of its 63 findings were provably finite loops outside the wave-1 grammar.

## Mental Model

Wave 1 shipped the exact tier: analyzers whose findings are true by construction. Wave 1.5
extends that tier where a termination proof exists (the S02 grammar widenings), and opens the
advisory on-ramp for rules that need judgment (`declorder`, `declusedistance`). The severity
ladder is the tuning loop: a heuristic rule enters advisory, accumulates trial evidence, and is
promoted to blocking by a one-line registry edit once its findings are all actionable.

`//tiger:batched` moves from validated-but-inert to consumed: it is the sanctioned answer for
per-item IO the world forces on the code, and — new in this wave — for the loop bound that IO
implies (the cursor waiver, below). It remains wave 1.5's only escape hatch. The generic
deviation directive and the TS-D06 ratchet do not ship: no *blocking heuristic* rule ships, so
the ADR-0003 trigger condition ("heuristic rules that create genuine false-positive pressure")
is not met. `ioinloop` blocks, but is built exact-by-construction (conservative classifier), so
it creates no false-positive pressure by design.

## Correctness Constraints

### State Invariants

1. **Every blocking finding is true by construction.** Violated by: admitting a heuristic shape
   into a blocking rule's grammar. On violation: it is an analyzer bug (wave-1 constraint 7),
   fixed in the analyzer, regression-pinned in the corpus. Each S02 grammar admission in this
   wave carries a termination proof (see Functional); `ioinloop` only flags calls it can prove
   are IO by package identity.
2. **Every escape directive in the tree surfaces as a standing advisory finding on every run.**
   The `directives` analyzer's `TS-L09-escape` finding is independent of the analyzers that
   consume the directive; consuming `//tiger:batched` in `ioinloop`/`boundedloop` must not
   suppress it. Violated by: coupling consumption to reporting. Enforced structurally: the two
   live in separate analyzers with no shared state.
3. **The directive vocabulary is closed.** `openenum` joins as an intent declaration; an unknown
   verb remains a blocking TS-L09 error. Enforced by `internal/directive`'s closed vocabulary
   slice and its tests.
4. **Every registered rule has a corpus with failure, compliant, and (where coverage is cut)
   known-miss cases.** Enforced by the existing meta-tests in `internal/rules/meta_test.go`,
   which derive from the registry and so cover the new rules with no new wiring.
5. **Severity lives only in the registry** (ADR-0002). New analyzers carry no severity, exit,
   or output logic.

### Behavioral Constraints

- **`//tiger:batched` never waives TS-S02 outside the cursor shape.** A `for {}` or arbitrary
  unbounded loop with a `batched` annotation still fires S02. The waiver is shape-gated so the
  directive cannot become a blanket infinite-loop suppression.
- **Tuning is monotone-lenient.** Every grammar widening in this wave removes findings and
  never adds one. Verified by the trial-pin reruns: per-rule counts may only decrease, and the
  named real bugs must still fire.
- **Determinism holds**: two identical runs produce byte-identical output.
- **A partial run is never presented as a complete run** (unchanged driver contract).

## Acceptance Criteria

Trial-pin numbers are targets recorded in the wave report, not automated gates; reruns are
manual (querator pin `1fd1bb2`, git-server scratch).

1. Querator TS-S02 drops from 37 findings to ≤5, with the pause/shutdown deadlock
   (`internal/handlers.go:117`) still firing. Git-server TS-S02 drops from 26 to ≤7, with all
   4 real bugs still firing — including the annotated-tag peel-chain CPU-spin DoS
   (`internal/graphapi/resolve.go:114`).
2. Querator's `requestLoop` (shutdown-request channel) and `stopCleanup` (closed-channel
   broadcast) pass TS-S03/TS-C05 with no querator code changes; the 5 real C05 findings
   (missing cancellation on client waits, `internal/logical.go`) still fire.
3. Git-server's `PackReason` passes TS-S08 once marked `//tiger:openenum`; the two real S08
   bugs (`writeHunk` missing default, `packCode` invalid nibble) still fire.
4. `ioinloop` fires on querator's 30 `continue nextBatch` per-id validation loops; adding
   `//tiger:batched <reason>` to a loop silences TS-M10 (and TS-S02 where the loop matches the
   cursor shape) while the TS-L09-escape standing advisory still surfaces for it.
5. Repo health: tiger self-check green, corpus and meta-tests green, golangci plugin parity
   green, byte-identical double run.

## Scope

### In Scope

1. `ioinloop` analyzer (TS-M10, blocking) consuming `//tiger:batched`.
2. TS-S02 bound-grammar extensions: compound `&&`/`||`, monotone counters, tuple-post
   reversal; plus the shape-gated `//tiger:batched` cursor waiver.
3. A second recognized shutdown shape for TS-S03/TS-C05: `struct{}`-element channels, with a
   shutdown-name fallback.
4. `//tiger:openenum` intent directive and its TS-S08 consumption.
5. `declorder` analyzer (TS-N06, TS-L05 — both advisory).
6. `declusedistance` analyzer (TS-S13, advisory).
7. The assert-adoption playbook document.
8. Spec amendments and system-doc updates for every behavior change above.

### Out of Scope / Non-Goals

- **`assertdensity` / `invariantsymmetry` analyzers.** Both trials showed their findings block
  on target codebases adopting an assert package first (querator: 30 findings all gated on it).
  The playbook is the deliverable; the analyzers wait for evidence it worked.
- **The generic deviation directive (`//tiger:<rule-id>`) and the TS-D06 ratchet.** No blocking
  heuristic ships, so per ADR-0003 neither mechanism is triggered. They ship together, in the
  first wave that promotes a heuristic rule to blocking.
- **Interface fan-out dedup.** Both trials showed one decision counted 4–5× across interface
  implementers (five backends × one method signature; affects N07/E06-class rules, none in this
  wave). Deduplication needs cross-package analysis the architecture deliberately avoids —
  recorded in Limitations with the evidence, deferred.
- **The 16 unranked deferred analyzers** (`paradigm`, `queuebound`, `unitsuffix`, `outptr`,
  `passthrough`, `corpuscheck`, `fuzzcoverage`, `adapters`, `ownership`, `escapecheck`,
  `canonical`, `wiretypes`, `hotpath`, `nofloat`, `quantitycast`, `domaintypes`). No trial
  evidence ranks them; several (`ownership`, `escapecheck`, `hotpath`) are SSA-chain work the
  wave-1 blueprint routes elsewhere.
- **`noabbrev` / `restatement`** — removed from the wave by the ticket; blocked on the ENG-153
  per-repo allowlist infrastructure, to be re-proposed explicitly if that lands.
- **A machine-parseable (JSON) output mode.** The git-server double-count was a grep-methodology
  bug (substring match on message text); diagnostics already carry structured categories. Noted
  as a future tooling item only.

## Dependencies and Constraints

- ENG-161 (remove TS-N12/N13/N15, relax TS-T06) is merged; this branch starts at that merge.
- Analyzers remain pure single-package `go/analysis` passes: no `Requires`, no `FactTypes`, no
  SSA (ADR-0002, enforced by the driver's assertion).
- Escape hatches remain governed by the ADR-0003 admission test.
- The `assert` package and invariant vocabulary exist in this repo and are unchanged here.

---

## Functional

### 1. `ioinloop` (TS-M10, blocking)

Fires on an IO call lexically inside a `for`/`range` body, at the call site, category `TS-M10`.

**IO classifier (contract).** A call is IO when its callee — function or method — is defined in
a package on the IO allowlist, resolved through `pass.TypesInfo` (the `*types.Func`'s
`Pkg()`), never by name matching. The seed list is stdlib packages whose operations are
unconditionally IO — `os`, `net`, `net/http`, `database/sql`, `syscall` — with the exact seed
finalized by the implementor against corpus cases. The list is extensible per-repo via an
analyzer flag (`-ioinloop.packages`, the same mechanism as `-participle.allow`), which the
`tiger check` driver already re-exposes automatically. Deliberately excluded: interface-based
detection (`io.Writer` implementers include `bytes.Buffer` — in-memory, not IO; flagging it
would break the exact-tier invariant).

**Known misses (marked in corpus):** IO behind an unlisted package (e.g. a gRPC client), IO in
a helper function called from the loop (no call graph in single-package analysis), IO in a
`FuncLit` defined in the loop (func literals are frame boundaries, consistent with
`deferdistance`).

**`//tiger:batched` consumption.** The directive attaches to a loop statement: it lives in the
comment group immediately preceding the loop (or on its line). It waives all TS-M10 findings in
that loop's own body, excluding nested loops — an inner loop with IO needs its own directive,
so accounting stays per-decision. The `directives` analyzer's TS-L09 validation (verb known,
reason present) and TS-L09-escape standing advisory are unchanged.

### 2. TS-S02 bound-grammar extensions (`boundedloop`)

Three exact widenings of `boundedCond`, each with a termination proof:

- **Compound conditions.** `A && B` is bounded if either operand is bounded (the loop exits
  when the bounded operand goes false). `A || B` is bounded only if *every* operand is bounded.
  Recurses through parentheses and `!`. Covers Myers-diff and two-pointer-merge shapes
  (6 git-server findings).
- **Monotone counters.** A condition comparing a single identifier against zero is bounded
  when every assignment to that identifier inside the loop (post *and* body) provably moves it
  to the exit: `x > 0`/`x >= c` with only decrements (`x--`, `x -= positive-const`); `x != 0`
  with only right-shifts or divisions by a constant > 1 (a shift provably reaches zero; a bare
  decrement under `!= 0` does not qualify — a negative start would wrap). If any assignment in
  the loop moves the identifier the wrong way or is unprovable, the loop is not bounded.
  Covers varint-over-finite-buffer (5 git-server findings).
- **Tuple-post reversal.** A multi-assignment post (`i, j = i+1, j-1`) where the condition
  relates two of the assigned identifiers (`i < j`) and the assignments provably close the gap.
  Extends `postCounter` beyond single-identifier increments. Covers the querator reversal
  shapes (4 findings).

**Cursor waiver (heuristic admission, escape-gated).** A loop matches the *cursor shape* when
its condition is a boolean method call on an identifier (`for it.Valid()`, `for rows.Next()`),
and either the call itself advances the cursor or the identifier receives a method call in the
post or body (the advance). A cursor-shaped loop annotated `//tiger:batched <reason>` is exempt
from the bound requirement: the store's finiteness is the same fact of the world that admitted
`batched` for TS-M10, and cursor drains do per-item IO by definition, so one directive with one
reason covers both rules on the same loop. Without the directive, the loop still fires S02.
Non-method-call pagination shapes (e.g. a loop-carried token) stay outside the waiver — a
documented known miss; those sites either restate a bound or restructure. This requires a spec
amendment: TS-S02's "no directive waives it" gains the cursor-shape exception, recorded with an
amendment note per the spec's convention. Covers 37 findings across both trials.

### 3. TS-S03/TS-C05 second shutdown shape (`boundedloop`, `selectctx`)

A *shutdown channel* is recognized when either:

- **Type-based:** the channel's element type is `struct{}` (a pure signal — the closed-channel
  broadcast shape, e.g. querator's `stopCleanup`); or
- **Name-based fallback:** the channel expression's identifier or selected field name,
  lowercased, contains `shutdown`, `stop`, `quit`, or `done` — regardless of element type. This
  is what covers a request-carrying shutdown channel (querator's `shutdownCh`, whose
  acknowledged-drain contract is stronger than `ctx.Done()`).

Consumption, in both analyzers' checks:

- `boundedloop` (S03): a `for {}` select satisfies the event-loop shape when any comm case
  receives from a `.Done()` call *or* a recognized shutdown channel.
- `selectctx` (C05): a select needs a `default`, a `.Done()` case, or a shutdown-channel case;
  a bare receive from — or bare send to — a recognized shutdown channel is exempt from the
  wrap requirement, the same exemption a bare `<-ctx.Done()` gets today (the shutdown handoff
  *is* the termination path).

The name fallback only relaxes: a false name-match silences a finding (a known-miss class,
marked in corpus), never creates one. Both recognitions get spec amendment notes on TS-S03 and
TS-C05.

### 4. `//tiger:openenum` (directive + TS-S08 in `compoundcond`)

New intent declaration, no arguments, placed in the doc-comment group of a named type
declaration. Semantics: the type's vocabulary is deliberately extensible, so a switch over it
requires a `default` arm (the value that came off the wire) but the default is a legitimate
catch-all — `assert.Unreachable` is not required and exhaustiveness is not expected.

Recognition is same-package only: `compoundcond` sees directives on type declarations in the
package under analysis; a switch in a different package from the marked type is a documented
known miss (single-package analysis cannot read another package's comments). Git-server's
`PackReason` switch is same-package, so the acceptance criterion holds.

Interaction note for docs: the auto rule half of TS-S08 (the `exhaustive` linter) has its own
opt-out (`//exhaustive:ignore`); `openenum` governs only tiger's custom default-arm check. The
system docs must state both, or an adopter marks the type and still gets auto findings.

Spec changes: `openenum` joins the Declarations table as an intent declaration bound to TS-S08;
TS-S08 gains the open-enum paragraph with an amendment note.

### 5. `declorder` (TS-N06 advisory, TS-L05 advisory)

- **TS-N06** (helper prefixed with its caller's name): enforced in the spec's own stated
  partial form — only for unexported functions with a single caller in the package. Category
  `TS-N06`.
- **TS-L05** (struct order: fields, nested types, constructor, methods): checks declaration
  ordering around each struct type. Category `TS-L05`.
- **TS-L04 is deliberately not shipped**: the spec marks it subsumed by TS-K04, which belongs
  to the `canonical` analyzer (chain 6, out of this wave).

Both advisory: ordering is judgment-adjacent, and neither has trial evidence yet. This wave's
trial rerun doubles as their first tuning data.

### 6. `declusedistance` (TS-S13, advisory)

Fires when more than 10 lines (the spec's calibration constant) separate a variable's
declaration from its first use within the declaring frame. The `deferdistance` lessons apply
directly: a `FuncLit` is a frame boundary (a use inside a nested closure counts as one use at
the closure's position; distance is never measured across frames), and a bare method call on
the variable counts as a use. Category `TS-S13`, advisory per spec ("Partial, advisory").

### 7. Assert-adoption playbook

A document, not an analyzer: `docs/Assert Adoption Playbook.md`. Contents: how a target
codebase adopts the assert/invariant pattern — introduce the `assert` package (tiger's own or
the vendored pattern), route naked panics through it (TS-S18), give closed-set switches
`assert.Unreachable` defaults (TS-S08), then declare the `inv` invariant vocabulary — and the
measurable state ("S18/S08 findings resolved without behavior change") at which
`assertdensity`/`invariantsymmetry` become worth building. Written from the trial evidence:
querator's 30 gated findings are the worked example; git-server's 4 test-only S18 findings show
the prerequisite is codebase-dependent.

### 8. Promotion criteria (registry policy, recorded here and in system docs)

An advisory rule is promoted to blocking when trial runs on at least two real codebases show
every finding actionable (no shape the trial judges a false positive), and no open tuning items
remain against the rule. Promotion is a one-line registry severity edit, with the evidence
linked in the promoting PR. Demotion (blocking → advisory) is allowed under the same evidence
bar in reverse and is likewise a registry edit.

## Architecture

No new subsystems. The wave is: three modified analyzers (`boundedloop`, `selectctx`,
`compoundcond`), three new analyzer packages (`ioinloop`, `declorder`, `declusedistance`), one
directive vocabulary entry (`openenum`, `KindIntent`), four new registry entries, corpus
additions, and documents.

**Registry additions** (`internal/rules/rules.go`):

| Rule ID | Category | Analyzer | Severity |
|---|---|---|---|
| TS-M10 | TS-M10 | ioinloop | Blocking |
| TS-N06 | TS-N06 | declorder | Advisory |
| TS-L05 | TS-L05 | declorder | Advisory |
| TS-S13 | TS-S13 | declusedistance | Advisory |

The CLI, plugin, and meta-tests all derive from the registry; no driver changes are needed
beyond what registration brings. The severity partitioning, exit codes, and
`markAdvisory` output rewrite are unchanged.

**Directive attachment (shared contract).** Both `ioinloop` and `boundedloop` resolve
`//tiger:batched` from the comment group attached to the loop statement (immediately preceding
lines or same line), parsed through `internal/directive`. `compoundcond` resolves
`//tiger:openenum` from the type declaration's doc-comment group the same way. Directive
*validation* stays solely in the `directives` analyzer; consumers parse but never report
TS-L09 findings (domain-boundary opacity: one analyzer owns the vocabulary's enforcement).

**Domain boundaries.** Analyzers stay severity-free and driver-agnostic (ADR-0002). The
`internal/directive` package remains the single owner of the vocabulary; consumers depend on
its `Parse`/`Lookup` contract, never on comment-text conventions of their own.

### Invariant Preservation

- *Blocking findings true by construction*: every S02 widening admits only shapes with a
  termination argument stated in this document; the cursor shape is the sole heuristic
  admission and it is inert without a visible, reasoned, standing-advisory escape. `ioinloop`'s
  package-identity classifier cannot flag non-IO; its unprovable cases are silent known misses.
- *Escapes never silent*: consumption and reporting live in different analyzers with no shared
  state; the corpus for `ioinloop` includes a case asserting that an annotated loop still
  yields the TS-L09-escape finding under the `directives` analyzer.
- *Corpus completeness*: the meta-tests derive from the registry, so the four new entries are
  covered the moment they register; the known misses named in Functional each get a marked
  `knownmiss*.go` file.

### Illegal State Analysis

The registry's coherence meta-test structurally rejects: an unregistered category, a rule
without a corpus directory, an analyzer package that does not exist, duplicate categories.
Application-logic enforcement (needs test coverage, not structure): the shape gate on the
cursor waiver, the monotone-counter "every assignment moves toward exit" check, and the
nested-loop exclusion in `batched`'s waiver span.

## Testing

Testing follows the `surface-testing` skill. No clocks, no external systems, no async behavior
— every surface is a pure function of source text.

Key surfaces:

- **Per-analyzer corpus via `analysistest`** (the analyzer's real consumer):
  - `ioinloop/testdata/src/ts-m10/`: failure (stdlib IO in loop), compliant (batched loop,
    hoisted IO, IO outside loops), known-miss (unlisted package, helper call, FuncLit).
  - `boundedloop/testdata/src/ts-s02/`: new compliant cases for all three widenings and the
    annotated cursor loop; failure cases for `||` with one unbounded arm, `!= 0` with bare
    decrement, cursor shape *without* the directive, non-cursor loop *with* the directive;
    known-miss for the loop-carried pagination token.
  - `ts-s03` / `ts-c05`: compliant cases for `struct{}` channels and shutdown-named channels
    (select case, bare receive, bare send); failure case for an unrecognized data channel;
    known-miss for a false name-match silencing a real finding.
  - `ts-s08`: compliant case for a marked open enum with a plain default; failure cases for a
    marked type with *no* default and for an unmarked closed set; known-miss for the
    cross-package switch.
  - `declorder`/`declusedistance` corpora per their rules, including frame-boundary cases
    ported from the `deferdistance` corpus patterns.
- **`internal/directive` unit tests**: `openenum` parses, formats, round-trips; is
  `KindIntent`; requires no reason.
- **Meta-tests**: unchanged; cover the new registrations automatically.
- **CLI fixture** (`internal/cli/testdata`): a `-ioinloop.packages` flag case proving the
  per-repo extension works through the driver.
- **Plugin parity**: existing smoke fixture extended with one TS-M10 case.
- **Trial-pin reruns**: manual, recorded in the wave report against the acceptance targets.

## Limitations & Future Work

- **Interface fan-out dedup** (deferred): one design decision still counts 4–5× across
  implementers. Evidence: querator `continue nextBatch` ×4 backends, git-server `SetConfig` ×5
  implementers. Needs cross-package attribution the single-package architecture forbids;
  revisit when a wave ships a signature-class rule.
- **Deviation directive + TS-D06 ratchet**: deliberately unbuilt; they ship together with the
  first blocking heuristic rule (ADR-0003).
- **Cross-package `openenum`**: a switch outside the marked type's package is unseen.
- **No call graph**: IO hidden behind a same-package helper escapes `ioinloop`.
- **Pagination shapes beyond the method-call cursor** stay outside the waiver.
- **JSON output mode**: future tooling; categories are already structured for machine use.
- **`assertdensity`/`invariantsymmetry`**: build only after the playbook shows a codebase
  crossing the adoption bar.

## Open Questions

- Final `ioinloop` seed list (implementor's call against corpus cases — e.g. whether
  `golang.org/x/sys/unix` joins `syscall`).
- Whether the shutdown-name fallback list (`shutdown`, `stop`, `quit`, `done`) needs a flag or
  stays a constant until a trial says otherwise.
