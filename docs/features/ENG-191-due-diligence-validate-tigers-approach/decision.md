# ENG-191 Decision: what tiger's static analysis can hold, and the rule set that follows

Date: 2026-09-24

Tiger's goal is AI-written Go that either conforms to a strict dialect or produces a blocking
finding the agent must fix. This document answers the four questions ENG-191 asks, in the order
it asks them, and ends each answer with the calls a reviewer has to ratify or veto. Every call opens with a
recommendation and says what ratifying and vetoing each mean. It then states what the rule checks,
shows it firing on real code from the two trial codebases with the exact text tiger prints, shows
what ratifying changes, and says what a veto costs.

Each recommendation is one of five kinds. **Keep** leaves a rule as it ships. **Change** keeps a
rule blocking but changes what it checks. **Remove** deletes a rule. **Reject** turns down a
proposed addition, so tiger stays as it is. **Fix docs** corrects the specification or explainer
without touching the tool.

A rule code like TS-S02 is a label. The sentence next to it says what the rule checks.

## The calls in brief

| # | Proposal | Recommendation | If you ratify | If you veto |
|---|---|---|---|---|
| 1 | Add a language model to judge waiver reasons | Reject | tiger stays deterministic; nothing changes | a model judges reasons inside `tiger check` |
| 2 | Stop blocking loops tiger can't prove end (TS-V01) | Change | 62 findings go away; written annotations are still checked | the rule keeps blocking as today |
| 3 | Remove the single-implementation interface rule (TS-X01) | Remove | the rule is deleted | 24 findings stay, each with a fix that worsens the design |
| 4 | Stop requiring a comment on dropped errors in cleanup code (TS-E02) | Change | 47 findings go away | a comment is still required there |
| 5 | Don't count a leading `t` or `ctx` toward the four-parameter limit (TS-N07) | Change | 21 findings go away | the limit counts them as today |
| 6 | Stop checking test files for goroutine-per-item loops (TS-C09) | Change | 12 findings go away | test files keep firing |
| 7 | Make four rules check behavior instead of names | Change | renaming no longer passes; correct `WaitGroup` code passes | the name holes stay open |
| 8 | Teach the map-order rule collect-then-sort (TS-T02) | Change | correct code stops firing | correct code must be rewritten to `slices.Sorted` |
| 9 | Add a baseline file that grandfathers existing findings | Reject | nothing changes | build a baseline mechanism |
| 10 | Make `//nolint` on a tiger rule a blocking finding (TS-L09) | Change | the silent bypass closes | `//nolint` keeps silencing tiger's rules |
| 11 | Keep `//tiger:restrict` opt-in and restore it in the explainer | Keep | the explainer gets the directive back | restrictions turn on by default |
| 12 | Fix the stale map-order wording in the spec and explainer | Fix docs | the spec and explainer are corrected | the documents stay inconsistent |

## The evidence this rests on

Every number below comes from building `tiger` at `5126797` (current `main`; this branch changes
only documents) and running `tiger check ./...` against the two trial pins.

- **querator** at `1fd1bb2`, the pin ENG-148 used. About 36k lines, a queue service with four
  storage backends.
- **git-server** at `321da03` in the mono-repo, the pin ENG-159 used, after `task duh-gen` and
  `task proto-gen` generate its protobuf code. About 17k lines, a git smart-HTTP server.

querator prints `tiger: 401 blocking` and git-server prints `tiger: 339 blocking`. Five of
querator's lines and 16 of git-server's are accounting findings (skipped tests, a misordered
declaration, and the missing budget rows for them), which print as blocking only because neither
repo has a `tiger.budget.yaml` yet. That file is the package budget. It is checked in, lists how
many waivers and skipped tests each package may carry, and fails the run when a package goes
over. Only a human edit can raise a number. One `tiger budget --write` records today's counts and
clears these lines. That leaves 396 and 323 rule findings.

Two fixture modules were also run against the same binary, to test claims the trials could not
answer on their own. They are reproduced inline where they matter.

---

## 1. Is static analysis enough for tiger's goal?

For most of it, yes. Static analysis gets tiger's goal where the rule checks a shape the code
either has or lacks. It fails where the rule has to prove something about runtime behavior with a
grammar smaller than the code people write, and where the rule is really a judgment about naming
or design. The truth of what a developer declares (why this loop is safe, why this error can be
dropped) is not a static question. Review already owns it.

**Shape rules work.** A shape rule classifies every instance of a construct as allowed or not. "Does
this loop state a bound?" has an answer for every loop. "Does this loop halt?" does not. Every one
of the ten real bugs both trials found came from a shape rule (the table in section 2). Shape rules
print byte-identical output on every run. A false positive on a blocking rule counts as a bug in
tiger's analyzer. Tiger fixes the analyzer, and no one adds a comment to silence the finding. On
both trials that held, with eight analyzer defects fixed and zero suppressions.

**Opt-in pins work.** Tiger computes some facts for every function and package without being
asked. Effects are which kinds of IO a function performs. Frames are which state it writes.
Restriction axes are what a package promises not to do. None of these produce a finding until
someone freezes one with a pin comment like `//tiger:effects`. After that, tiger checks the pin
against the code in both directions. It fires when the code does more than the pin says, and when
the pin claims more than the code does. A codebase that never pins anything gets zero noise, which
is how a whole-program analysis ships without flooding an adopter.

**Judgment rules do not work, and are already gone.** Rules that judged names against a dictionary
(TS-N12, N13, N15), flagged helpers with one caller (TS-N06), or measured distance between a
declaration and its use (TS-S13) were removed after both trials showed their output was mostly
domain vocabulary and shapes the metric could not tell from real mistakes. ADR-0006 records the
removal. The specification keeps those maxims and names review as their enforcement. Nothing here
argues for bringing them back.

**Termination proofs beyond the shape do not work as blocking rules.** TS-V01 tries to prove every
loop ends by finding a value that shrinks on every pass. Its grammar recognizes a handful of forms
(`i < n` with `i++`, `len(s) > 0` with `s = s[1:]`, and a few more). Real loops outgrow it
immediately. Section 3, call 2 has the evidence and the fix.

**Whether a declaration is true is not static at all.** Whether a `//tiger:batched` reason is
honest, whether a type really is an open enum, whether the comment beside `_ = f.Close()` is
correct. The specification's "what no amount of tooling fixes" table puts these with review, and
that is right. Tiger's job is to keep them visible on every run and impossible to add without a
diff. It should not judge them.

**Runtime checks back up the static rules.** The spec's double-run test (TS-T11 runs each test
twice and diffs the output), `goleak`, `-race`, and mutation score all catch what a static shape
only approximates. Map order is the example. TS-T02 bans the shape, and the double run proves the
output never varied.

### Call 1: add a language model to judge waiver reasons

**Recommendation: Reject.** If you ratify, tiger stays deterministic and nothing changes. If you
veto, a model judges waiver reasons inside `tiger check`.

**What it governs.** ENG-191 asks whether some goals need review by a language model. The only
place a model could plug in is the free-text reason a developer writes on a waiver. Tiger never
interprets free text. It checks that a reason is present, and nothing more.

**Where it comes up today.** A cursor loop in querator's Postgres backend, written the way ADR-0004
allows (the probe from section 3, call 2):

```go
//tiger:batched rows arrive from a Postgres cursor; the table size is the bound
for rows.Next() {
```

Tiger checks two things here. The text after `batched` is not empty, and the loop has a cursor's
shape (a boolean method call like `rows.Next()` that advances the cursor). If both hold, the loop
bound is waived, the waiver is printed on every run, and it counts against the package budget.
The same input gives the same result on every run. Whether the table really is finite is a
judgment, and it happens in code review of the pull request, not in `tiger check`.

**What ratifying changes.** Nothing. Tiger stays deterministic. The reason stays a claim a
reviewer, human or AI, reads on the pull request.

**What a veto costs.** A model inside the pass/fail verdict makes it nondeterministic. The same
commit could pass on one run and fail on the next, and an agent could rephrase a reason until the
model agrees. A green run is the one thing an agent cannot currently argue with, and a model in
the verdict would make it negotiable. A model reviewing the pull request, outside the verdict, gets
every benefit with none of that cost.

---

## 2. What the trial evidence says

### What found real bugs

Across ENG-148 (querator), ENG-159 (git-server), and their reruns in ENG-149 and ENG-162, tiger
found ten real defects. All ten came from five blocking shape rules.

| Rule, in plain words | Bugs | What it caught |
|---|---:|---|
| TS-S02, every loop states a bound tiger can see (a constant, a `len`, or a counter) | 5 | a pause/shutdown deadlock; a remotely triggerable CPU spin through an uncapped annotated-tag peel chain; two uncapped pkt-line command loops; an uncapped ref-update accumulation |
| TS-S08, a switch over an enum handles every value or ends in `assert.Unreachable` | 3 | two storage backends silently dropping an unknown action; a diff writer that emits nothing for a future op tag; a pack writer returning an invalid type code |
| TS-C05, every blocking channel wait can be cancelled | 1 | three client waits with no cancellation, where the doc comment claimed there was |
| TS-C02, every goroutine is started by something that waits for it | 1 | an unwaited `WaitGroup` letting `Shutdown` return before its goroutines exited |
| TS-E02, a discarded error (`_ =`) needs a comment saying why | 1 | a hash-format validation error discarded, which would have produced corrupt object ids the day a second hash format shipped |

### What today's tiger prints

Counts per rule on each pin. The last column says what reading the findings showed.

| Rule | querator | git-server | What the findings are |
|---|---:|---:|---|
| TS-N07 parameters | 45 | 112 | mostly adjacent `string` params, a real swap hazard; see call 5 for the cap half |
| TS-E02 discarded error | 86 | 42 | 47 are cleanup inside `defer` or `t.Cleanup`; see call 4 |
| TS-E06 at most two return values, the second an `error` or `bool` | 11 | 39 | true by spec |
| TS-V01 loop proof | 37 | 25 | no bugs; see call 2 |
| TS-S02 loop bound | 35 | 23 | 30 querator cursor drains, which `//tiger:batched` waives; the deadlock bug still fires |
| TS-T06 test has a doc comment | 40 | 2 | true |
| TS-S09 no labeled `continue`/`break` | 35 | 0 | 30 copies of one `continue nextBatch` idiom across four mirrored backends |
| TS-T02 map order | 11 | 17 | mixed; see call 8 |
| TS-X01 single-impl interface | 9 | 15 | 24 of 24 are deliberate seams; see call 3 |
| TS-S08 closed switch | 15 | 14 | true; needs an assert package |
| TS-S18 no bare `panic` outside the assert package | 15 | 4 | true; needs an assert package |
| TS-C02 goroutine owner | 14 | 1 | 10 querator hits are correct `WaitGroup` supervision; see call 7 |
| TS-C05 cancellable wait | 13 | 0 | the real missing-cancellation waits still fire |
| TS-C09 goroutine per item | 12 | 0 | all in `_test.go`; see call 6 |
| TS-N08 no `bool` parameter | 5 | 12 | mixed, mostly true |
| TS-S06, S01, M10, T10, C12, N14 | 13 | 17 | true by spec; querator's two TS-S01 hits are real recursion through the request loop |

### What a static rule could never catch

The trial reports and this rerun agree on three things tiger cannot see:

- Whether a waiver's reason is true. A `//tiger:batched` on an infinite generator shaped like a
  cursor passes, as ADR-0004 says.
- Whether an interface is the right seam. git-server's `RefReader` exists to make ref mutation
  impossible from the graph API. No shape rule can tell that interface from a speculative one
  (call 3).
- Order leaking through a path the allowlist does not model, such as collecting map keys into a
  slice and never sorting it. The double-run test catches that one.

### The largest gap: rules that trust names

Four blocking rules decide compliance from an identifier's name where the behavior is computable.
A fixture run against today's binary shows it. This code passes `tiger check` with exit 0:

```go
type Latch struct{}

func (Latch) Done() <-chan bool { return nil } // never fires

func DrainFake(l Latch, work <-chan int) int {
	var total int
	for {
		select {
		case <-l.Done():
			return total
		case n := <-work:
			total += n
		}
	}
}

type Worker struct {
	shutdown chan bool // nothing ever closes or sends on this
	work     chan int
}

func (w *Worker) Run() int { /* same loop, selecting on w.shutdown */ }

func (b *Buf) Reset() {} // clears nothing

func Use(p []byte) int {
	b, ok := pool.Get().(*Buf)
	...
	b.Reset()
	pool.Put(b)
	return n
}
```

Rename `Done` to `Fired`, `shutdown` to `events`, and drop the empty `Reset` call, and the same
code fails:

```
probe.go:16:3: TS-C05: this select blocks with no case that ends the wait on shutdown — add case <-ctx.Done(): return ctx.Err()
probe.go:16:3: TS-S03: this event loop's select has no case that stops the loop — add case <-ctx.Done(): return, or a case on a shutdown channel (struct{}-typed or named like one), so the loop can end
probe.go:35:3: TS-C05: this select blocks with no case that ends the wait on shutdown — add case <-ctx.Done(): return ctx.Err()
probe.go:35:3: TS-S03: this event loop's select has no case that stops the loop — add case <-ctx.Done(): return, or a case on a shutdown channel (struct{}-typed or named like one), so the loop can end
probe.go:63:10: TS-M05: this Put is not preceded by a Reset (or a zeroing) of the same value on every path to it — call Reset() or zero the value before every Put, so pooled data can't leak to the next user
tiger: 5 blocking
```

An agent optimizing for a green run finds these holes first. They do not show that the approach
is wrong. They show where a rule checks a name instead of the behavior it is meant to check, so
code that breaks the rule passes. Call 7 closes them.

---

## 3. The rule set tiger should ship

Ten rules change, one is removed, and the rest stay as they are. Nothing
new enters as a rule. The golangci-lint linters tiger turns on are out of scope. The
`tiger golangci` audit governs them, and neither trial found a problem with that split.

### Call 2: stop blocking loops tiger can't prove end (TS-V01)

**Recommendation: Change.** If you ratify, 62 findings go away and a written `//tiger:variant` is
still checked. If you veto, the rule keeps blocking as it does today.

**What the rule enforces.** TS-V01 requires every loop to carry a proof that it ends. Tiger looks
for a value that shrinks on every pass and has a floor, and fires when it cannot find one. A
developer can supply the value with `//tiger:variant <expr>`, and tiger checks it. This is
separate from TS-S02, which only asks that a loop state a bound (a constant, a `len`, or a
counter) without proving the bound is reached.

**Where it fires today.** git-server's sideband writer. The loop ends because `chunk` is never
empty while `data` is non-empty, so `data` shrinks by at least one byte every pass. TS-S02 accepts
it (the condition is a `len`). TS-V01 does not:

```go
func writeSideband(buf *bytes.Buffer, channel byte, data []byte) {
	for len(data) > 0 {
		chunk := data
		if len(chunk) > maxSideband {
			chunk = chunk[:maxSideband]
		}
		fmt.Fprintf(buf, "%04x", len(chunk)+5)
		buf.WriteByte(channel)
		buf.Write(chunk)
		data = data[len(chunk):]
	}
}
```

```
services/git-server/internal/negotiate/pktline.go:36:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
```

Adding the correct pin does not help:

```
services/git-server/internal/negotiate/pktline.go:37:2: TS-V01: //tiger:variant len(data) doesn't provably shrink on every pass (len(data) cannot be shown to move consistently) — add a cap (for tries := 0; tries < max; tries++) that fails when hit, or name an expression that does shrink
```

The same happens on querator's cursor loops. ADR-0004 admitted `//tiger:batched` to waive TS-S02
on a database cursor, because the only honest bound is the size of the table. With the waiver in
place, TS-S02 goes quiet and TS-V01 still blocks:

```go
//tiger:batched rows arrive from a Postgres cursor; the table size is the bound
for rows.Next() {
```

```
internal/store/postgres.go:400:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
```

TS-V01 fires 37 times on querator and 25 on git-server. On querator, 30 of the 37 are loops
TS-S02 also flags, all cursor drains. On git-server, 21 of 25 are loops TS-S02 already accepts,
so TS-V01 is the only rule blocking them. I read all 21. Every one terminates. They are
slice-consuming parsers, `for size > 0 { size >>= 7 }` varint encoders, Myers diff index walks
(`for x > 0 && y > 0 { x--; y-- }`), worklists guarded by a visited set, and a delta resolver that
returns an error when a pass makes no progress. Across both codebases TS-V01 caught nothing TS-S02
did not. It also misses the one real termination bug in the trials, a `for { select }` loop, which
is outside its scope by design.

**What ratifying changes.** Tiger still works out the termination proof where it can and shows
it with `--show-facts`, and `tiger pin` can lock it in. A written `//tiger:variant` is still
checked in both directions. A loop with no annotation that TS-S02 accepts is no longer a finding. TS-S02 stays the blocking rule for termination, with its
ADR-0004 cursor waiver intact.

Before, on the sideband writer: one blocking finding, and no compliant exit except an artificial
cap. After: no finding. A developer who wants the proof on record writes a pin tiger can verify,
and if tiger cannot verify it, that pin is still a finding.

**What a veto costs.** 62 blocking findings across the two codebases stay, none of them a bug. The only
exit the rule offers is an iteration cap like `for tries := 0; tries < max; tries++`. For cursor
loops, ADR-0004 rejected that cap as "either a magic number or a restatement of however big the
store is". For parsers and encoders it is a number nobody can justify. The `//tiger:batched`
waiver ADR-0004 admitted stays half-broken, clearing one rule and leaving the loop blocked by
another. The explainer's claim that synthesis "covers nearly every real loop" stays false on both
codebases it has been measured against.

### Call 3: remove the single-implementation interface rule (TS-X01)

**Recommendation: Remove.** If you ratify, the rule is deleted. If you veto, 24 findings stay, each
with a suggested fix that makes the design worse.

**What the rule enforces.** TS-X01 fires on any interface that has exactly one implementation
outside `_test.go` files, on the theory that such an interface is abstraction added just in case.
Its message tells the developer to delete the interface and use the concrete type.

**Where it fires today.** git-server's graph API reads refs through a deliberately narrow
interface:

```go
// RefReader is the ref half of the seam graphapi consumes: listing and resolution
// only. It is deliberately narrower than storage.RefStore — CompareAndSwap is not
// in the surface, so a graph query structurally cannot mutate refs (I2).
type RefReader interface {
	List(ctx context.Context, prefixes []string) ([]storage.Ref, error)
	Resolve(ctx context.Context, name string) (oid storage.OID, ok bool, err error)
}
```

```
services/git-server/internal/graphapi/graphapi.go:32:6: TS-X01: interface RefReader has one implementation, memory.refStore — use memory.refStore directly and delete the interface, or add a second implementation outside _test.go files
```

Following that message gives the graph API a type with `CompareAndSwap` on it, which breaks
invariant I2. The other 14 git-server hits are the storage ports (`Backend`, `RefStore`,
`ObjectStore`, `PackEngine`, `GraphIndex`) that a cross-adapter conformance suite exists to test,
plus the event `Emitter` seam and the `Repos` and `RepoPolicy` seams between packages. querator's
9 hits are role interfaces like `QueueAdmin` and `UsersAdmin` over one `service.Service`, where
the fix widens every consumer's dependency to the whole service. That is 24 findings, all of them
deliberate seams, and following the rule's own suggested fix would make every one of them worse.

**What ratifying changes.** ADR-0006 removes a rule "whose output on real codebases is
dominated by findings a reader would decline to act on, including any finding whose named remedy
would break the code". TS-X01 meets that on both codebases. Removal is end to end, meaning the
analyzer, its test cases, its registry entry, and the specification's enforcement line. The
maxim stays in the specification with review as its enforcement.

The first pass of this document proposed an advisory trial instead, pending a git-server rerun.
That rerun is above, and it answered the question the trial would have asked.

**What a veto costs.** 24 blocking findings stay whose named fix is a design regression, including one
that would delete a structural safety guarantee. An agent told to reach green follows the message.

### Call 4: stop requiring a comment on dropped errors in cleanup code (TS-E02)

**Recommendation: Change.** If you ratify, 47 findings go away. If you veto, a comment is still
required on every dropped error in `defer` and `t.Cleanup`.

**What the rule enforces.** TS-E02 fires on `_ = f()` and `x, _ := f()` when the discarded value
is an error, unless a comment on that line or the line above explains why.

**Where it fires today.** querator's benchmarks and git-server's tests:

```go
defer func() { _ = d.Shutdown(ctx) }()
```

```
benchmarks/benchmark_main_test.go:37:19: TS-E02: this error is discarded with _, so a failure here goes unnoticed — handle the error, or say in a comment on this line or the line above why ignoring it is safe
```

```go
t.Cleanup(func() { _ = client.Close(context.Background()) })
```

That cleanup shape is 36 of querator's 86 TS-E02 findings and 11 of git-server's 42. The
compliant fix is always the same comment, something like `// best-effort cleanup`. The rule
checks only that a comment exists, never what it says. A fixture confirms that `// x` satisfies
it, and so does a bare `//nolint` (see call 10).

**What ratifying changes.** A discard inside a `defer` statement's call, or inside a function
literal passed to `t.Cleanup`, is not a finding. Every other discard still fires, including the
hash-validation discard that was the rule's one real bug, and non-deferred cleanup like
`_ = resp.Body.Close()`.

Before:

```go
defer func() { _ = d.Shutdown(ctx) }() // best-effort cleanup
```

After:

```go
defer func() { _ = d.Shutdown(ctx) }()
```

ADR-0006 allows tuning when the misfires form a bounded shape that can be named and excluded.
"Inside `defer` or `t.Cleanup`" is that kind of shape.

The exemption has a cost. `defer func() { _ = f.Close() }()` on a file opened for writing drops
the write error, and that is a real bug class. The rule does not catch it today either. It asks
for a comment, and the comment can say anything.

**What a veto costs.** 47 boilerplate comments across two codebases, and the rule keeps teaching agents
that any comment clears it.

### Call 5: don't count a leading `t` or `ctx` toward the four-parameter limit (TS-N07)

**Recommendation: Change.** If you ratify, 21 findings go away. If you veto, the limit keeps
counting `t` and `ctx`.

**What the rule enforces.** TS-N07 has two halves. It fires when two adjacent parameters share a
type, because Go has no named arguments and a caller can swap them. It also fires when a function
takes more than four parameters.

**Where it fires today.** querator's test helpers:

```go
func assertPartition(t *testing.T, ctx context.Context, c *querator.Client, name string, expected Partition) {
```

```
service/partition_test.go:503:1: TS-N07: this function takes 5 parameters — group the related ones into an options struct
```

The two leading parameters are Go convention and cannot be swapped with anything after them. They
use up half the cap before the helper does anything.

**What ratifying changes.** A leading `testing.TB` (or `*testing.T` or `*testing.B`) and a
`context.Context` that comes first or right after it do not count toward the cap. The
adjacent-same-type half is untouched, and that half carries the swap hazards, like the 67
adjacent-`string` findings on git-server.

Before, `assertPartition` counts 5 and fires. After, it counts 3 and passes. Across the two
codebases, 9 of querator's 10 cap findings and 12 of git-server's 21 clear. The ones that remain
are real, like an eight-parameter `Finalize` in querator's benchmarks and an eight-parameter diff
helper in git-server.

**What a veto costs.** 21 findings whose fix is an options struct wrapping `name` and `expected`, in helpers
where nobody would swap them. Agents learn to thread `t` through a struct field, which reads worse.

### Call 6: stop checking test files for goroutine-per-item loops (TS-C09)

**Recommendation: Change.** If you ratify, 12 findings go away and production code is checked as
before. If you veto, test files keep firing.

**What the rule enforces.** TS-C09 fires on a loop that starts one goroutine per item, because in
production that lets outside traffic set the concurrency level. Its fix is a bounded queue drained
by one worker.

**Where it fires today.** Only in tests. Every one of querator's 12 findings is the fan-out-then-wait
idiom a concurrency test uses on purpose:

```go
var wg sync.WaitGroup
wg.Add(len(requests))

for i := range requests {
	go func(idx int) {
		defer wg.Done()
		if err := c.QueueLease(ctx, requests[idx], responses[idx]); err != nil {
			...
		}
	}(i)
}
```

```
service/common_test.go:626:3: TS-C09: this loop starts a goroutine per item, so the goroutines react to each item instead of working at their own pace — append the items to a bounded queue and drain it from one supervisor goroutine
```

The test exists to put N concurrent requests on the server at once. A single-worker queue would
serialize them and defeat the test.

**What ratifying changes.** TS-C09 skips `_test.go` files. Production fan-out still fires.
There were zero production hits on either codebase, so the production behavior is unchanged.

**What a veto costs.** 12 findings whose fix breaks the tests they fire in.

### Call 7: make four rules check behavior instead of names (TS-C02, S03, C05, M05)

**Recommendation: Change.** If you ratify, renaming no longer passes these rules and correct
`WaitGroup` code passes. If you veto, the name holes stay open.

**What the rules enforce, and where each trusts a name.**

- **TS-C02**: every `go` statement starts through a supervisor, meaning something that waits for
  the goroutine and ties it to a context. Today the only recognized supervisor is
  `errgroup.Group.Go`, since ENG-153 removed the config flag that named supervisor functions.
- **TS-S03**: a loop that runs forever selects on a case that can stop it. **TS-C05**: a blocking
  channel wait has a case that can cancel it. Both accept `<-x.Done()` for any `x` with a method
  named `Done`, and any channel whose name contains `shutdown`, `stop`, `quit`, or `done`.
- **TS-M05**: a value put back in a `sync.Pool` is reset first. It accepts any method named
  `Reset`, including an empty one.

**Where they fire, or fail to.** The fixture in section 2 shows the three name holes passing. For
TS-C02 the problem runs the other way. querator's daemon supervises its server goroutine
correctly, and the rule cannot see it:

```go
d.wg.Add(1)
go func() {
	defer d.wg.Done()
	...
	if err := srv.ServeTLS(d.Listener, "", ""); err != nil {
```

```
daemon/daemon.go:175:2: TS-C02: this go statement starts a goroutine nobody owns: nothing says when it exits or ties it to a context — start it through errgroup.Group.Go
```

Ten of querator's 14 TS-C02 findings are this shape. The other four are real, and one of them is
the trial's unwaited-`WaitGroup` bug.

**What ratifying changes.** ENG-177 comes back as written. Each rule recognizes behavior tiger
can already compute:

| Rule | Today accepts | After, accepts |
|---|---|---|
| TS-C02 | only `errgroup.Group.Go` | also `wg.Add` before the `go` with `wg.Wait` in the same function or in the owning type's `Close`/`Shutdown` |
| TS-S03, TS-C05 | any `.Done()` | `.Done()` only on a `context.Context` |
| TS-S03, TS-C05 | any channel named like `shutdown` | a channel the package closes or sends on somewhere |
| TS-M05 | any method named `Reset` | a `Reset` that writes the receiver |

The fixture's `DrainFake`, `Worker.Run`, and `Use` would each fire. `daemon.go:175` would pass.

**What a veto costs.** An agent gets past three blocking rules by naming a channel `done` or writing
`func (b *Buf) Reset() {}`. Ten correct supervisions on querator have no compliant path except a
rewrite to `errgroup`. This is the largest gap between the promise that blocking rules are exact
and what tiger actually ships.

### Call 8: teach the map-order rule collect-then-sort (TS-T02)

**Recommendation: Change.** If you ratify, correct collect-then-sort code stops firing. If you veto,
that code must be rewritten to `slices.Sorted(maps.Keys(m))`.

**What the rule enforces.** TS-T02 bans ranging over a map unless the loop body matches an
allowlist of order-insensitive shapes (summing, counting, writing into another map). Go randomizes
map order on every run, and the rule keeps that randomness out of anything the program emits.

**Where it fires today.** querator's in-memory user store collects ids and sorts them before use:

```go
ids := make([]string, 0, len(m.users))
for id := range m.users {
	ids = append(ids, id)
}
sort.Strings(ids)
```

```
internal/store/memory.go:1054:2: TS-T02: this loop appends to a slice while ranging over a map, and Go visits map entries in a different order every run, so the slice's order varies — range over the sorted keys instead: for _, k := range slices.Sorted(maps.Keys(m))
```

I sampled four of querator's 11. One is a real order leak into logs. Three are order-insensitive
shapes the allowlist does not model: this append followed by a sort, nested deletes on a second
map, and a map write under `if`/`else`.

**What ratifying changes.** The analyzer extends its allowlist with those shapes, each backed
by a test case. The ENG-150 blueprint already says the allowlist grows this way, as an
analyzer change and never a config knob. The first shape to add is an append into a local slice
that is sorted before any other use.

Before, the snippet above fires. After, it passes, and the same loop without the `sort.Strings`
still fires.

**What a veto costs.** Developers rewrite correct code into the `slices.Sorted(maps.Keys(m))` form. That
is harmless but pure churn. The rule is not wrong. It is narrower than it needs to be.

### Call 9: add a baseline file that grandfathers existing findings

**Recommendation: Reject.** If you ratify, nothing changes. If you veto, tiger gets a mechanism
that records today's findings and fails only on new ones.

**What it governs.** Some linters let an adopter record today's findings in a file and fail only
on new ones. Tiger has no such file for blocking rules.

**Where it bites.** A first run prints 396 rule findings on querator and 323 on git-server. An
existing codebase cannot adopt tiger incrementally. It has to fix everything or disable
analyzers.

**What ratifying changes.** Nothing. This records the cost instead of paying it with a
mechanism. ADR-0011 refused the same mechanism for advisory findings, because a file that raises
a limit is the cheapest path to green an agent can find. The argument is the same for blocking
findings. Tiger's stated target is new AI-written code, where this cost does not arise.

**What a veto costs.** A baseline lets brownfield teams adopt tiger in a day. It also gives an agent a
command that turns any regression green. If the project wants brownfield adoption, that deserves
its own ADR, with a design that can only ever lower the numbers, like ADR-0011's budget file.

### Rules that stay as they are

These stay blocking with no change. Each was true on real code in both trials. Where the volume is
high, it counts one decision made many times, not many wrong decisions.

| Rule | What it checks | querator / git-server |
|---|---|---:|
| TS-S02 | every loop states a bound: a constant, a `len`, or a counter; `for {}` needs a `select` on `ctx.Done()` | 35 / 23 |
| TS-S03 | a loop that runs forever has a case that stops it | 0 / 0 |
| TS-S08 | a switch over an enum ends in `assert.Unreachable`, or the type is marked `//tiger:openenum` | 15 / 14 |
| TS-S09 | no labeled `break` or `continue`; move the inner loop into a function | 35 / 0 |
| TS-S18 | no bare `panic` outside the assert package | 15 / 4 |
| TS-S01 | no recursion; a cycle in the call graph is a finding | 2 / 3 |
| TS-S06, S07 | one logical operator per condition; split compound assertions | 2 / 6 |
| TS-S21, S22 | every limit constant is asserted against, and its stated derivation evaluates to its value | 0 / 0 |
| TS-C05 | every blocking channel wait can be cancelled | 13 / 0 |
| TS-C12 | channel types are declared in one file per package | 3 / 0 |
| TS-M05 | a pooled value is reset before `Put` (gets call 7's fix) | 0 / 0 |
| TS-M10 | no IO inside a loop body, unless `//tiger:batched` | 2 / 6 |
| TS-E06 | at most two return values, the second an `error` or `bool` | 11 / 39 |
| TS-N08 | no `bool` parameters; use a named type | 5 / 12 |
| TS-N14 | exported names do not end in a present participle | 0 / 1 |
| TS-T06, T10 | tests have a doc comment; table tests have a `name` field | 44 / 3 |
| TS-L09 | every directive is well formed and carries a reason | 0 / 0 |
| TS-F01, F02, F07, V03, P01, P02, K03, A07, A09 | opt-in pins (effects, frames, preconditions, invariants) and restrictions; silent until declared | 0 / 0 |

The accounting findings stay as they are. TS-D07 is a skipped test, TS-L05 a misordered
declaration, TS-L09-escape an escape directive in use, and TS-D06 the budget that counts them.
Together they printed 21 lines across the two codebases before `tiger budget --write`, and they are
the surface a reviewer checks.

---

## 4. The three explainer disagreements

Writing the ENG-154 explainer asserted three product decisions that the spec, code, and ADRs do not
reflect. Each resolves below as part of the rule set above, not as a separate patch.

### Call 10: make `//nolint` on a tiger rule a blocking finding (TS-L09)

**Recommendation: Change.** If you ratify, the silent bypass closes and `//nolint` for other
linters is counted. If you veto, `//nolint` keeps silencing tiger's rules.

**What the three sources say.** The specification allows `//nolint` with a reason, enforced by
golangci-lint's `nolintlint` under TS-L09. The explainer says tiger "flags any `//nolint` as a
finding". The code does neither.

**What the code does.** Under `tiger check`, `//nolint` is not read at all, so it silences nothing
by itself. It does satisfy any rule that asks for a comment. A fixture shows it on TS-E02:

```go
_ = os.Remove(path)
```

```
probe.go:8:2: TS-E02: this error is discarded with _, so a failure here goes unnoticed — handle the error, or say in a comment on this line or the line above why ignoring it is safe
tiger: 1 blocking
```

```go
_ = os.Remove(path) //nolint
```

That exits 0. Under the golangci-lint plugin, `//nolint:tiger` silences any tiger rule on that line
with no advisory and no count (ENG-178, item 2). The explainer's sentence is false about the tool
as shipped.

**What ratifying changes.** Tiger's own rules accept no suppression, whether tiger runs as
`tiger check` or inside golangci-lint. The
`directives` analyzer already reads every comment. A bare `//nolint`, or one that names `tiger`,
becomes a blocking TS-L09 finding. The finding is reported at the file's `package` line, where
the `//nolint` on the original line cannot cover it.

```
probe.go:1:1: TS-L09: //nolint at probe.go:8 would silence tiger's rules — fix the finding it hides; tiger's rules take no suppression
```

(The message wording is illustrative. ADR-0009 governs the final text.)

For the golangci-lint linters tiger turns on, `//nolint:<linter> // reason` stays
allowed. Several of those linters (`gocritic`, `gosec`, `mnd`) are heuristics with documented
false positives the project does not own, and ADR-0003 already concedes that door is outside
tiger's control. What changes is that it is counted. Each one is treated as a waiver. It is
printed on every run as a `TS-L09-escape` notice and counted against the package budget, like a
`//tiger:batched`.

After this, the explainer's sentence becomes true for tiger's rules, and it gains a clause for the
golangci-lint linters.

**What a veto costs.** Under the plugin, one comment silences any blocking rule with no trace. Under the
CLI, a `//nolint` clears any rule that asks for a comment, by accident. The explainer keeps promising a ban
that does not exist. The alternative, banning `//nolint` for every enabled linter, is defensible
and simpler to explain. It was not chosen because it forces adopters to turn off whole linters
over one false positive, which is louder but loses more signal than a counted waiver does.

### Call 11: keep `//tiger:restrict` opt-in and restore it in the explainer (TS-P01, P02, K03)

**Recommendation: Keep.** If you ratify, the tool stays as it is and the explainer gets the
directive back. If you veto, restrictions turn on for every package by default.

**What the rule enforces.** A package can declare restrictions in its doc comment, such as
`//tiger:restrict closed-dispatch` (every method call resolves to one concrete type, never through
an interface) or `no-reflect`. TS-P01 checks the package's own imports against the declaration.
TS-P02 checks that every dependency declares at least as much. TS-K03 flags interface calls in a
closed-dispatch package. A package that declares nothing gets no finding.

**What changed in the explainer.** Commit `b452818` removed every mention of the directive, and
ENG-188 asserted that `no-reflect` and `closed-dispatch` "are not opt-in", which implies they are
on for every package.

**What closed dispatch by default would cost.** Declaring it on querator's `internal` package
alone produces 68 blocking findings:

```
internal/auth_backend.go:104:37: TS-K03: a.roleBindings.ListByUser is called through interface RoleBindings in a package that declares //tiger:restrict closed-dispatch — call the concrete type's method, or drop closed-dispatch
internal/auth_backend.go:4:1: TS-P02: package internal claims closed-dispatch but imports github.com/cespare/xxhash/v2, which declares nothing — add //tiger:restrict closed-dispatch to github.com/cespare/xxhash/v2, or drop closed-dispatch from internal's declaration
```

26 are calls through `Partition`, the interface querator's four storage backends implement. That
interface is the architecture. Twelve are calls on `context.Context`, including `ctx.Done()`,
which TS-S03 and TS-C05 require. And TS-P02 fires on every third-party import, since third-party
code declares nothing. A default-on closed-dispatch rule contradicts tiger's own cancellation
rules and cannot be satisfied by any codebase that imports a library.

**What ratifying changes.** The shipped design stays. The restriction set is an opt-in claim a
package makes, and absence is never a finding. The explainer restores the paragraph that
introduces the directive. Without it, a reader who meets TS-P01 in the rule reference has no way
to learn what a restriction set is. The explainer was half right about one axis. Reflection is
already banned everywhere by the golangci-lint linter behind TS-S12, and `no-reflect` on the directive only
makes that claim checkable by dependents. The explainer should say so.

**What a veto costs.** Either the rule reference documents three blocking rules the explainer never
mentions, or restriction becomes default-on and every adopter meets findings like the 68 above on
day one.

### Call 12: fix the stale map-order wording in the spec and explainer (TS-T02)

**Recommendation: Fix docs.** If you ratify, the specification and explainer are corrected and the
tool is unchanged. If you veto, the documents stay inconsistent with each other and the code.

**What the three sources say.** The explainer says the map-order check bans every map range except
a fixed set of safe body shapes, and calls it "a heuristic rather than a proof". The
specification's analyzer table says the analyzer flags a "range over a map whose body appends or
writes. Heuristic". The code does what the explainer describes, the allowlist inversion shown in
call 8. The ENG-150 blueprint made that inversion on purpose, called it exact, and flagged the
specification line for amendment. The amendment never landed.

**What ratifying changes.** Nothing in the product changes. This is a disagreement between documents only.

- The specification's TS-T02 enforcement line and its Part V analyzer table change to describe
  the allowlist.
- The explainer drops "heuristic". When the rule fires, the loop really does have a shape outside
  the safe list; it never guesses. It is not a proof that order never reaches an output, because
  one shape it deliberately ignores (collect the keys into a slice, never sort, then range the
  slice) gets past it. The replacement wording is "exact over a
  conservative allowlist, backstopped by the double-run test (TS-T11)".

**What a veto costs.** The specification keeps describing an analyzer tiger does not ship, and the
explainer keeps calling a rule that never guesses a heuristic, which invites readers to treat its findings as
optional.

---

## What happens next

### The canceled tickets

Eleven backlog tickets were canceled while this question was open. The direction holds, so most of
them describe work this decision still wants. Reopening is for whoever ratifies this. Nothing here
reopens a ticket.

| Ticket | Disposition |
|---|---|
| ENG-177, recognize behavior, not names | revive as written (call 7) |
| ENG-178, close uncounted silencing channels | revive items 2, 3, 4: `//nolint` under the plugin (call 10), recognize `package assert` by import path, drop the unread `hot`/`wire`/`owner` verbs. Drop item 1; one advisory per generated file is noise of the kind ADR-0006 removes |
| ENG-179, trusted declarations get the escape treatment | revive item 1 (`//tiger:openenum` counted as an escape with a reason). Drop item 2; call 4's cleanup exemption replaces the proposed `//tiger:discard` directive |
| ENG-176, revive naming rules on the config file | leave canceled; scoping was never the failure |
| ENG-175, exemptions for signatures pinned by third-party interfaces | leave canceled; one site in one repo, and an adapter function is the compliant shape |
| ENG-188, reconcile spec with explainer | superseded by this document and its follow-ups |
| ENG-181, specification gaps | fold into the specification follow-up below |
| ENG-174, JSON output | revive when convenient; both trial reports asked for it |
| ENG-171, 172, 173, auto-fix, editor, dashboards | leave canceled until the rule set above settles |

### Follow-ups, in order

1. **TS-V01 stops blocking unannotated loops** (call 2). Turn the test cases for unannotated
   loops into passing cases, then rerun both codebases and record the counts.
2. **ENG-177, all four items** (call 7). A test case per recognized shape, and a test showing each
   shape the rule deliberately ignores. Rerun both codebases.
3. **Remove TS-X01** (call 3), end to end per ADR-0006.
4. **Exemptions for TS-E02, TS-N07, TS-C09** (calls 4, 5, 6), each with a test case for the
   exempted shape.
5. **Silencing channels** (call 10). ENG-178 items 2 to 4, ENG-179 item 1, and counting
   `//nolint:<linter>` as `TS-L09-escape`.
6. **TS-T02 allowlist growth** (call 8), starting with collect-then-sort.
7. **Specification reconciliation**, one ticket. Covers the TS-T02 line and Part V table, TS-V01,
   TS-X01's removal, TS-L09's `//nolint` wording, the restriction-set defaults, and the ENG-181
   gaps (`tiger.yaml`, `tiger golangci --print`, the rule count).
8. **Explainer reconciliation**, one ticket. Restore the `//tiger:restrict` paragraph, split the
   `//nolint` sentence into the tiger-rule ban and the counted waiver for golangci-lint linters, replace
   "heuristic" on map order, remove "covers nearly every real loop", and restore the fuller
   built-versus-described disclaimer that `b452818` shortened.
9. **An ADR** recording the rule behind call 2. When a rule needs a proof tiger cannot always
   build, tiger shows the proof as information and checks it only where someone writes it down,
   never as a finding on code nobody annotated. This document is the context and the ADR is the binding record.
