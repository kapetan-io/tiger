# ENG-148 — Trial: wave-1 tiger on querator

Wave-1 tiger (`tiger check` and the golangci-lint module plugin) ran against
[querator](https://github.com/kapetan-io/querator) at `1fd1bb2` — a real
distributed queue: ~36k lines of hand-written Go plus ~6k of protoc-generated
code, four storage backends, a request-loop core, and a large concurrent test
suite. Every finding class was verified against the analyzer's own
implementation and corpus by independent review passes; a false positive on a
blocking rule was treated as an analyzer bug and fixed in the analyzer with a
corpus case, never suppressed (correctness constraint 7).

Headline: the first run reported **1023 blocking + 39 advisory** findings.
After fixing seven analyzer defects the same tree reports **880 blocking +
2 advisory** — a 17% reduction with zero suppressions, and two rules
(TS-L10, TS-N14) went from 56 and 11 findings to zero, every one of them a
false positive. Output was byte-identical across repeated runs, before and
after the fixes.

## Analyzer bugs found and fixed (all in commit `4072d9a`)

1. **Driver: no generated-file exclusion.** 117 findings (59 E06, 44 N12,
   7 N14, 7 N15) landed in `proto/*.pb.go` — files carrying
   `// Code generated ... DO NOT EDIT.` that no one can hand-edit, where the
   diagnostics' own remedies are incoherent. The driver now drops findings
   in generated files (`ast.IsGenerated`); the files still load and
   type-check. This also restores parity with the plugin path, where
   golangci-lint's default `generated: lax` exclusion was silently doing
   this already — the two drivers disagreed by 117 findings.
2. **deferdistance: func literals treated as transparent.** 19/19 of the
   blocking "defer inside a loop" findings — 100% of the rule's blocking
   output — were defers inside `go func(){}` or subtest closures spawned by
   a loop, which resolve once per frame and queue nothing across
   iterations. 12/37 advisory findings were the same confusion (a closure's
   `defer wg.Done()` has no local acquisition to sit next to). Func
   literals are now frame boundaries for both halves.
3. **deferdistance: a method call is not an acquisition.** `mu.Lock();
   defer mu.Unlock()` — the most idiomatic acquire/release pair in Go —
   fired "sits away from its acquisition" because only assignments and
   declarations counted. A bare method call on the target now counts (this
   also silences the `now.Freeze(...)`/`defer now.UnFreeze()` test-fixture
   idiom, 20 findings).
4. **namedeny: `-allow` did not fold plurals.** The deny dictionary folds
   (`item` denies `item` and `items`) but the allow list didn't, so
   silencing querator's domain noun required two entries and the
   diagnostic's own suggested remedy (`-allow=items=<reason>`) left half
   the findings firing. Allow entries now fold both ways.
5. **participle: genuine nouns flagged.** `RoleBinding` (an association
   record) and `ProduceWaiting` (a count of waiters) are the same class as
   `finding`/`blocking`, which entered the allowlist from tiger's own
   dogfood run. `binding` and `waiting` added; 11 findings, all false.
6. **boundedloop: TS-S03 fired on non-blocking selects.** A `for { select
   { ... default: return } }` drain loop cannot hang, and sibling rule
   TS-C05 already exempts exactly this shape — the two analyzers disagreed
   on the same AST. `checkEventLoop` now recognizes a default clause.
7. **selectctx: TS-C05 flagged `<-ctx.Done()` itself,** telling the author
   to wrap the shutdown wait in a select on `ctx.Done()` — a no-op
   restatement of the code already there. A bare receive from a `Done()`
   call is now exempt.

Also fixed: **nogoto's message overclaimed.** A labeled `break` whose label
marks its only enclosing loop (Go requires the label because a bare `break`
exits the `select`/`switch`) was told it "reaches across loops" when no
second loop exists. The rule still fires — the message now names what the
label actually does.

## Findings per rule (after fixes)

| Rule | Count | Verdict on real code |
| --- | --- | --- |
| TS-N12 empty name tokens | 388 | Fires as designed, but 76% is querator's own domain vocabulary: `item`/`items` **is** the queue's core noun, `info` is the `QueueInfo`/`PartitionInfo` descriptor suffix mirrored from the wire protocol, `handler` mirrors stdlib. One interface-method name multiplies ×5 across backends. The real signal (`manager`, `temp`, `data`) is a small minority. |
| TS-E02 discard needs comment | 86 | 100% true by spec; ~half is deferred `Close`/`Rollback`/`Shutdown` cleanup. Because any comment satisfies the shape check, the practical remediation is boilerplate comments, which undercuts the rule's intent. |
| TS-N13 type echo | 82 | 67 are the `...List` RPC-verb suffix of the public API (`QueuesList`, paired with `...Create`/`...Delete`), repeated per implementer — a naming convention, not the stale-type failure mode the rule targets. `connString`, `sigChan` are terms of art. True positives: `itemPtr`, `toStr`. |
| TS-T06 tests state their goal | 64 | 100% true by spec. 40 tests have no doc comment; 24 have real doc comments that lack the literal `Goal:` token — the report reads identically for both, which is the rule's harshest edge. |
| TS-N07 same-type params / max 4 | 45 | ~30 genuine swappable-parameter hazards (`namespace, userID, roleID string` across two backends is the poster child). ~15 are the `t, ctx, client` test-helper prefix burning the budget by convention. |
| TS-E06 return arity | 11 | Was 70; 59 were generated protobuf `Descriptor()` methods. Of the rest: 10 idiomatic multi-value helpers, and 1 **unsatisfiable** — a func literal assigned to third-party `porcupine.Model.Step`, whose signature querator cannot change. |
| TS-S02 loop bound | 37 | 1 real bug (below). 27 are DB cursor/iterator loops (`badger it.Valid()`, `rows.Next()`) — finite, store-bounded, invisible to a const/len/cap rule, and with **no wave-1 escape** (`//tiger:batched` targets TS-M10, not S02). 4 are provably-bounded shapes just outside the recognized grammar (tuple-post reversal, monotone `!= 0`). |
| TS-S09 no labeled break/continue | 35 | Zero gotos. 30 findings are one idiom — `continue nextBatch` in a nested per-id validation loop — copied across four mirrored backends. True by the exact rule; volume overstates decisions by ~4×. |
| TS-C05 blocking op without Done | 19 | 5 real (below). The rest: closed-channel shutdown (`stopCleanup`), buffered cap-1 handoffs, wall-clock test timeouts — safe idioms outside the literal-`.Done()` shape. |
| TS-S18 no naked panic | 15 | True by spec; all 15 blocked on the same prerequisite — querator has no `assert` package to route through. 5 are already exhaustive-switch guard arms, the exact shape `assert.Unreachable` exists for. |
| TS-S08 switch ends in Unreachable | 15 | True by spec, same assert-adoption prerequisite. **3 sites would change behavior if mechanically fixed** (a legitimate catch-all dispatch; two log-and-degrade defaults). Also surfaced a real gap (below). |
| TS-C02 goroutine via supervisor | 14 | 4 real (an unjoined `daemon.wg`, below). 10 are complete, idiomatic supervision — `wg.Add` / `Close()` calls `wg.Wait()` — that the nominal `-supervisors` allowlist has no vocabulary for. |
| TS-C09 no reactive spawning | 12 | 100% test-file noise: every finding is the bounded fan-out-then-`wg.Wait()` concurrency-test idiom. Zero production hits. |
| TS-N08 no bool params | 5 | 1 strong (`PauseQueue(ctx, name, true)` at call sites), 4 weak. |
| TS-T10 named table cases | 4 | True by spec; all four benchmarks synthesize names via `fmt.Sprintf` + `b.Run`, so output is readable despite the missing `name` field. |
| TS-S03 event loop selects Done | 3 | The remaining findings are querator's deliberate architecture: `requestLoop`'s `shutdownCh` carries an acknowledged-drain contract *richer* than `ctx.Done()`; `stopCleanup` is the closed-channel broadcast. The rule needs a second recognized shape, not a querator fix. |
| TS-C12 chan types in one file | 3 | Correctly computed, but satisfying it means moving the ADR-documented `Logical` actor into `auth_cache.go` — the rule scopes to "package" where this package is organized per-subsystem. |
| TS-S06 one operator per condition | 2 | True; both are the compact `(c < '0' \|\| c > '9') && (c < 'a' \|\| c > 'z')` character-class check, where the split arguably reads worse. |
| TS-N15 name pairs | 40 | 33 are the `in *proto.X, out *types.Y` converter convention used consistently across every proto↔domain function. 7 true positives (`srcID`, `srcList`). |
| TS-D07 skipped tests (advisory) | 2 | Both real `t.Skip()` guards in the fuzz harness; skipcheck correctly reached into the `f.Fuzz` closure. |
| TS-N14 participle | 0 | Was 11 — all false positives, fixed. |
| TS-L10 defer placement | 0 | Was 19 blocking + 37 advisory — all false positives or fixture idioms, fixed. |

## Real querator bugs the rules surfaced

The trial's strongest evidence that the dialect earns its keep — four
genuine defects found by blocking rules, worth querator tickets:

1. **Pause/shutdown deadlock** (TS-S02, `internal/handlers.go:117`):
   while paused, `handlePause` ranges over `requestCh` only; `Shutdown`'s
   send on `shutdownCh` has no listener, so shutdown times out and leaks
   the request loop.
2. **Missing cancellation on client waits** (TS-C05,
   `internal/logical.go:197/264/310/353`): only `Lease`'s `<-req.ReadyCh`
   is backed by a server-side `ctx.Err()` sweep; `Produce`, `Complete`,
   and `Retry` waits are not, though `ProduceDeadLetter` in the same file
   shows the fix and `Lease`'s doc comment claims the behavior generally.
3. **Silently dropped action** (TS-S08, `internal/store/mongo.go:1422`,
   `postgres.go:1646`): both switches lack `case ActionItemMaxAttempts`
   *and* a default — the action is dropped with zero logging, while badger
   and memory at least warn.
4. **`daemon.wg` is never `Wait()`'d** (TS-C02, `daemon/daemon.go`):
   `Shutdown` can return before the HTTP-serve goroutines exit.

## Adoption experience

- **Determinism holds**: `tiger check` twice over 49k LOC, byte-identical,
  before and after the fixes.
- **`tiger golangci` audit** against querator's existing `.golangci.yml`
  works well: 46 unenforced auto rules, each line naming the rule, the
  linter, and the exact config remedy; exit 1. `--init` correctly refuses
  when a config exists (exit 2) and points at the audit. **Gap**: there is
  no path from "audit says 46 missing" to a merged config — the user
  hand-edits 46 entries. An `--init` variant that prints the baseline for
  merging would close the loop. `--init` on a fresh module needs a `go.mod`
  (clear error without one) and its output immediately audits clean.
- **The escape hatch enforces its grammar**: `//tiger:batched <reason>`
  surfaces as a standing advisory on every run; dropping the reason or
  using an unknown verb is a blocking finding with a good message. But in
  wave 1 the directive **loosens nothing** — its target rule TS-M10
  (`ioinloop`) ships in wave 1.5 — so there is currently *no* escape for
  any wave-1 rule. On real code that bites hardest at TS-S02: a DB-cursor
  scan has no compliant rewrite short of an artificial cap.
- **Skip advisory works end to end**: both `t.Skip` sites found inside the
  `f.Fuzz` closure, reported on every run, exit code untouched.
- **Plugin parity has two silent divergences** (before the driver fix,
  three): golangci-lint's default `issues.uniq-by-line: true` drops all
  but one finding per line (six same-line TS-N07 findings become one), and
  golangci-lint's **analysis cache does not invalidate when the plugin
  binary changes** — after rebuilding `tiger-gcl` with fixed analyzers it
  served the old findings until `cache clean`. The plugin docs should
  prescribe `issues.uniq-by-line: false` and a `cache clean` after
  rebuilds.

## Ranked wave-1.5 priorities

Ranked by what the trial actually demonstrated, not by spec order:

1. **`ioinloop` (TS-M10)** — completes the only escape hatch. Wave 1
   validates and surfaces `//tiger:batched` but nothing consumes it; the
   per-item-IO-in-a-loop pattern it governs is querator's bread and butter
   (the 30 `continue nextBatch` sites live in exactly those loops), and
   shipping it gives the S02-adjacent iterator problem its sanctioned
   answer.
2. **A second recognized shutdown shape for S03/C05** (closed-channel
   broadcast, and/or a shutdown-request channel) — not a listed 1.5
   analyzer but the highest-leverage tuning the trial found: querator's
   core `requestLoop` implements a *stronger* contract than `ctx.Done()`
   and still cannot pass. ~17 findings, including the architecture the
   ADRs document.
3. **Bound-grammar extensions for S02** (tuple-post counters, monotone
   `!=`/`==` against zero, cursor-iterator recognition or a sanctioned
   cap convention) — 36 of 37 findings are legitimate finite loops the
   const/len/cap grammar cannot see. Without this (or ioinloop's escape),
   S02 cannot gate a storage-heavy codebase.
4. **`declorder` / `declusedistance`** — cheap wins on proven corpus
   conventions; the deferdistance frame-boundary lesson (closures are
   frames, method calls are acquisitions) applies directly to
   `declusedistance`'s design.
5. **`noabbrev` / `restatement`** — defer until allowlist infrastructure
   matures. The naming-dictionary rules were the trial's noise leaders
   (N12+N13+N15 = 58% of remaining findings, dominated by domain
   vocabulary and API conventions), and `-allow` as a single flag string
   does not scale to the per-repo dictionaries these rules need. A
   config-file allowlist (with reasons, reviewed like code) is the
   prerequisite.
6. **`assertdensity` / `invariantsymmetry`** — gate on an assert-adoption
   playbook. Querator has no `assert` package; 15 S08 + 15 S18 findings
   (and 3 sites where the mechanical fix would change behavior) all block
   on that adoption step, and density rules would only widen the gap.

Supporting infrastructure the trial argues for, independent of analyzer
order: per-repo allowlist files for the naming rules, `tiger golangci`
baseline-merge output, plugin documentation for `uniq-by-line` and cache
invalidation, and an interface-aware exemption story for signatures pinned
by external contracts (the `porcupine.Step` case is unfixable by design).
