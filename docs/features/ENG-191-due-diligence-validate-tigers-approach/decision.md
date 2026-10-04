# ENG-191 Decision: what tiger's static analysis can hold, and the rule set that follows

Date: 2026-09-24, updated 2026-10-01

Tiger's goal is AI-written Go that either conforms to a strict dialect or produces a blocking
finding the agent must fix. This document answers the four questions ENG-191 asks, in the order
it asks them, and ends each answer with the calls to decide. Every call opens with a
recommendation and says what happens if we agree and if we don't. It then states what the rule
checks, shows it firing on real code from the two trial codebases with the exact text tiger prints,
shows what agreeing changes, and says what not agreeing costs. A decided call carries a
**Decision** line naming the outcome.

Each recommendation is one of five kinds. **Keep** leaves a rule as it ships. **Change** keeps a
rule blocking but changes what it checks; a decided Change is recorded as *Keep and change*. **Remove** deletes a rule. **Reject** turns down a
proposed addition, so tiger stays as it is. **Fix docs** corrects the specification or explainer
without touching the tool.

A rule code like TS-S02 is a label. The sentence next to it says what the rule checks.

The 2026-10-01 update audited querator's 30 cursor-shaped loops, ran experiments on how tiger
checks loop bounds, and put the loop rules through three deliberations. That reversed call 2 (keep
requiring proof that a loop ends, and make both loop rules sound), added calls 13 and 14 in
section 5, turned call 14 into a tool change, and expanded call 3. A prototype of call 2's change 6
found three bugs in the trial code, filed as ENG-193, ENG-194 and ENG-195.

## The calls in brief

| # | Proposal | Recommendation | If we agree | If we don't | Decision |
|---|---|---|---|---|---|
| 1 | Add a language model to judge waiver reasons | Reject | tiger stays deterministic; nothing changes | a model judges reasons inside `tiger check` | Reject |
| 2 | Keep requiring proof that a loop ends, and make both loop rules sound (TS-V01, TS-S02) | Change | the proof rule keeps blocking and proves more; the 20 holes in `experiments/gaps` close; a constant cap reports when hit | 62 findings on correct code stay, and loops that run forever keep passing | Keep and change |
| 3 | Remove the single-implementation interface rule (TS-X01) | Remove | the rule is deleted | 24 findings stay, each with a fix that worsens the design | Remove |
| 4 | Stop requiring a comment on dropped errors in cleanup code (TS-E02) | Change | 47 findings go away | a comment is still required there | Keep and change |
| 5 | Don't count a leading `t` or `ctx` toward the four-parameter limit (TS-N07) | Change | 21 findings go away | the limit counts them as today | Keep and change |
| 6 | Stop checking test files for goroutine-per-item loops (TS-C09) | Change | 12 findings go away | test files keep firing | Keep and change |
| 7 | Make four rules check behavior instead of names | Change | renaming no longer passes; correct shutdown code passes; a loop's only exit is its stop case; goleak is required | the name holes stay open and the rules point agents at shapes that hid four querator bugs | Keep and change |
| 8 | Accept collect-then-sort in the map-order rule (TS-T02) | Change | correct code stops firing | correct code must be rewritten to `slices.Sorted` | Keep and change: plain sorts plus full coverage |
| 9 | Add a baseline file that grandfathers existing findings | Reject | nothing changes | build a baseline mechanism | Reject |
| 10 | Make `//nolint` on a tiger rule a blocking finding (TS-L09) | Change | the silent bypass closes | `//nolint` keeps silencing tiger's rules | Keep and change: opt-in, off by default |
| 11 | Keep `//tiger:restrict` opt-in and restore it in the explainer | Keep | the explainer gets the directive back | restrictions turn on by default | Remove all package-level declarations |
| 12 | Fix the stale map-order wording in the spec and explainer | Fix docs | the spec and explainer are corrected | the documents stay inconsistent | Fix docs: describe call 8's rule and its known misses |
| 13 | Stop using `//tiger:batched` to waive the loop bound (TS-S02, ADR-0004) | Change | store loops restate the limit they already declare; whole-partition scans get a declared maximum or cancellation | the waiver keeps standing in for a limit nobody declared | open |
| 14 | Tell a safety cap from a page size by the loop's other exit (TS-S02) | Change | a cap that guards an internal condition must report when hit; a page over an outside stream need not | the spec keeps asking for an assert the tool never checks, and silent caps keep passing | open |
| 15 | Ship a tested patterns collection for what the rules can't check | Change | runnable, CI-checked patterns back the rules where they stop short; rule messages and reviewers point to them | the unchecked parts of a correct shape stay with review, with nothing to compare against | Agreed: packages plus the binary |
| 16 | Replace effect and frame pins with a `tiger changed` report | Change | a reviewer sees which struct started calling something new, with nothing an agent can edit to hide it | pins that miss every database and network call through an interface or library stay | Keep and change: report replaces pins |

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
answer on their own. They are reproduced inline where they matter. Five experiment modules are
committed under [`experiments/`](experiments/). Four of them (`cursorbound`, `annotations`,
`variants`, `gaps`) hold loops with runtime tests and the captured output of `go test` and
`tiger check`. The fifth, `limitfacts`, is the prototype analyzer for call 2's change 6.

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

**Proving a loop ends works only when whatever stops it is in the code.** Calls 2, 13, and 14
follow from this line.

| The loop stops because of | Example | What tiger can prove |
|---|---|---|
| a value in the code that the body visibly shrinks | `for len(p) > 0 { p = p[1:] }` | it ends, within `len(p)` passes |
| a counter against a declared limit | `for n := 0; n < limit && rows.Next(); n++` | it runs at most `limit` times, whatever else happens |
| something outside the program: a database, the network, another goroutine | `for rows.Next()`, `for l.inFlight.Load() != 0` | nothing about termination; tiger can require a declared limit or a cancellation path (`ctx.Done()`), or record a reviewed claim |

Tiger's spec states the principle behind this as B1, taken from TigerStyle and, through it, from
NASA's Power of Ten rules: "Everything in reality has a limit. Code that does not declare its limit
has one anyway, chosen by whatever runs out first, discovered at the worst time." The goal is to
declare the limit. The counter is just where tiger can see it.

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

**Decision (2026-10-01): Reject.** Tiger stays deterministic. Waiver reasons stay a claim that pull request review reads, outside the pass/fail verdict.

**Recommendation: Reject.** If we agree, tiger stays deterministic and nothing changes. If we
don't, a model judges waiver reasons inside `tiger check`.

**What it governs.** ENG-191 asks whether some goals need review by a language model. The only
place a model could plug in is the free-text reason a developer writes on a waiver. Tiger never
interprets free text. It checks that a reason is present, and nothing more.

**Where it comes up today.** A cursor loop in querator's Postgres backend, written the way ADR-0004
allows (call 13 proposes replacing it):

```go
//tiger:batched rows arrive from a Postgres cursor; the table size is the bound
for rows.Next() {
```

Tiger checks two things here. The text after `batched` is not empty, and the loop has a cursor's
shape (a boolean method call like `rows.Next()` that advances the cursor). If both hold, the loop
bound is waived, the waiver is printed on every run, and it counts against the package budget.
The same input gives the same result on every run. Whether the table really is finite is a
judgment, and it happens in code review of the pull request, not in `tiger check`.

**What agreeing changes.** Nothing. Tiger stays deterministic. The reason stays a claim a
reviewer, human or AI, reads on the pull request.

**What not agreeing costs.** A model inside the pass/fail verdict makes it nondeterministic. The same
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
| TS-C02 goroutine owner | 14 | 1 | 4 querator hits are correct `WaitGroup` code, 3 are the unwaited-`WaitGroup` bug, 7 are other shapes; see call 7 |
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

### Call 2: keep requiring proof that a loop ends, and make both loop rules sound (TS-V01, TS-S02)

**Decision (2026-10-01): Keep and change.** TS-V01 keeps blocking and both loop rules get the soundness fixes, with changes 5 and 6 as decided in the deliberations on caps and limits.

**Recommendation: Change.** If we agree, a loop still fails the build when tiger cannot prove it
ends. Tiger learns enough new proofs to clear most of the 62 findings on correct code. Both loop
rules stop accepting the loops in `experiments/gaps` that run forever, and a cap that stands in for
a proof has to report when it is hit. If we don't, both loop rules stay as they are, including 62
findings that are not bugs and 20 loops tiger accepts that run forever, cut work short, or skip it.

An earlier version of this call recommended the opposite: stop blocking, and show the proof only as
information. The experiments in section 5 changed that. With only the bound rule blocking, loops that
run forever pass, so the proof rule stays blocking and the work goes into making it prove more. A
review of the rewritten call (three reviewers, each testing claims with probes) then found the holes
listed below, and every one now has a test.

**What the rule enforces.** TS-V01 requires every loop to carry a proof that it ends. Tiger looks
for a value that shrinks on every pass and has a floor, and fires when it cannot find one. A
developer can supply the value with `//tiger:variant <expr>`. That is a small arithmetic
expression (numbers, variables, and `len(...)`), not free text, and tiger checks it by the same
fixed rules it uses to find one itself (section 5 shows how). TS-S02 is the weaker rule. It asks
only that a loop's header state a bound (a constant, a `len`, or a counter). A loop with a counter
in its header is out of TS-V01's scope, so a counter cap is the way out when tiger cannot find a
proof. That makes the counter check as important as the proof itself.

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

Adding the correct annotation does not help. Tiger's checker only accepts a step of a fixed
number like `data[1:]`, so it cannot see that `data[len(chunk):]` always moves forward:

```
services/git-server/internal/negotiate/pktline.go:37:2: TS-V01: //tiger:variant len(data) doesn't provably shrink on every pass (len(data) cannot be shown to move consistently) — add a cap (for tries := 0; tries < max; tries++) that fails when hit, or name an expression that does shrink
```

The annotation is narrower than its grammar suggests. [`experiments/variants`](experiments/variants/)
writes eight variants, runs each through `tiger check`, and has `go test` run each loop
([test](experiments/variants/variants_test.go), [test output](experiments/variants/test.out),
[tiger output](experiments/variants/tiger.out)):

| Loop | Variant | Body | TS-V01 | TS-S02 | At runtime |
|---|---|---|---|---|---|
| Countdown | `i` | `for i > 0 { i-- }` | verified | passes | ends |
| Stack pop | `len(stack)` | `stack = stack[:len(stack)-1]` | verified | passes | ends |
| Index by 2 | `n - i` | `for i < n { i += 2 }` | verified | **blocks** | ends |
| Two pointers | `high - low` | `low++; high--` | verified | **blocks** | ends |
| Struct field | `len(q.Items)` | `q.Items = q.Items[1:]` | rejected | passes | ends |
| Wrong direction | `n - i` | `for i < n { i-- }` | rejected | blocks | runs forever |
| Varint | `size` | `size >>= 7` | rejected | passes | ends |
| Chunked slice | `len(data)` | `data = data[len(chunk):]` | rejected | passes | ends |

Tiger rejects the one false claim, and names why: "i decreases, growing the ranking instead of
shrinking it". It also rejects three true ones. It knows only fixed steps (`i--`, `i += 2`,
`s[1:]`), so a shift or a slice by `len(chunk)` is "cannot be shown to move consistently", and its
condition check accepts only plain local variables, so `len(q.Items) > 0` is "not one of the
recognized comparison forms". The varint and chunked shapes are git-server's.

The two rules also disagree with each other. TS-V01 verifies `n - i` and `high - low`, and TS-S02
still blocks both loops because `i < n` and `low < high` have no counter in the loop header:

```
variants.go:24:6: TS-S02: tiger can't tell how many times this loop runs: its condition is not a constant, a len, or a counter — add a cap (for tries := 0; tries < max; tries++) that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
```

A developer who writes a correct variant, and gets it verified, still has a blocking finding.

**Why the proof rule should keep blocking.** TS-S02 alone accepts loops that run forever. The
annotations experiment in section 5 has two. One appends back into the slice its condition
measures, and the other skips the shrink with `continue`. Both have a `len` condition, so TS-S02
accepts them, and both are still running when the test stops waiting. TS-V01 blocks both. Writing
this call up found a third case. TS-S01's own documented replacement for recursion is a worklist
whose counter chases a slice the body grows:

```go
stack := []*Node{root}
for i := 0; i < len(stack); i++ {
	stack = append(stack, stack[i].Next...)
}
```

On a graph with a cycle, `stack` grows as fast as `i` advances, so the loop never ends. Tiger
reports nothing, because TS-S02 sees a counter and TS-V01 skips counter loops.
[`experiments/gaps`](experiments/gaps/) holds this loop and a test that shows it running forever
on a two-node cycle ([test](experiments/gaps/gaps_test.go), [test output](experiments/gaps/test.out),
[tiger output](experiments/gaps/tiger.out)). It also holds the `ctx.Err() == nil` game loop from
change 6, which stops on cancel exactly like the `select` loops tiger accepts. git-server's tag-peel
denial of service was this kind of walk.

TS-V01 fires 37 times on querator and 25 on git-server. On querator, 30 of the 37 are store and
cursor loops TS-S02 also flags. Most of those do not depend on this call, and call 13 covers them.
On git-server, 21 of 25 are loops TS-S02 already accepts, so TS-V01 is the only rule blocking them. I read all 21. Every one terminates. They are
slice-consuming parsers, `for size > 0 { size >>= 7 }` varint encoders, Myers diff index walks
(`for x > 0 && y > 0 { x--; y-- }`), worklists guarded by a visited set, and a delta resolver that
returns an error when a pass makes no progress. Across both codebases TS-V01 caught nothing TS-S02
did not. It also misses the one real termination bug in the trials, a `for { select }` loop, which
is outside its scope by design.

**What the review found.** Both rules accept loops that never end. TS-S02 trusts any counter whose
name appears in the condition. TS-V01 matches variables by name and looks only inside the loop
body. A cap is accepted at any size and stops without telling anyone. Each loop below is in
[`experiments/gaps`](experiments/gaps/), with a test that runs it
([tests](experiments/gaps/holes_test.go), [more tests](experiments/gaps/holes2_test.go),
[test output](experiments/gaps/test.out), [tiger output](experiments/gaps/tiger.out)).

| Hole | Loops | Tiger today | At runtime |
|---|---|---|---|
| TS-S02 trusts any counter | `i < 10 \|\| !done()`; `i = 0` or `i--` in the body; `n++` on the limit; `r.i < 10` with a local `i`; `i != 7` stepping by 2; `uint8` counted to `<= 255`; a counter moving away from its limit | no finding | run forever |
| TS-V01 matches by name, inside the body only | a shadowing copy shrinks instead; growth through a pointer or a closure made before the loop | no finding | run forever |
| TS-V01 ignores wraparound | `i <= math.MaxInt` with `i++` | TS-S02 only; TS-V01 verifies it | runs forever |
| A growing limit is invisible | TS-S01's worklist; growth through a helper, a method, or `len(map)` | no finding | run forever |
| A cap passes at any size | `i < math.MaxInt && !done()` | no finding | runs forever |
| A cap stops silently | `i < len(stack) && i < nodesMax` on a cycle | TS-S21 on the constant only | returns 101 nodes as if complete |
| Range dodges both rules | `for _, n := range stack` while appending to `stack` | no finding | visits only the root |
| Iterators are trusted | `for x := range forever()` | no finding | runs forever |
| `ctx.Err()` without a cancel | `for ctx.Err() == nil` on `context.Background()` | blocked today; change 4 as first drafted would pass it | runs forever |

`goto` loops and `continue outer` from an inner loop also skip a shrink, but TS-S09 already blocks
both, so the loop rules do not need to handle them.

Two measurements shaped the plan. The growing-limit check (change 3) hits 0 loops in querator and 0
in git-server, whose worklists all pop with `for len(queue) > 0`. It hits 14 loops in tiger's own
code, all correct walks over syntax trees, plus TS-S01's compliant test fixture. And change 2 makes
TS-S02 trust whatever TS-V01 proves, so a hole in TS-V01 would clear both rules at once.

**What agreeing changes.** TS-V01 keeps blocking, and a written `//tiger:variant` is still checked
in both directions. All of the following ship in one release, with the soundness fixes in place
before TS-S02 starts trusting a proof.

1. **TS-V01's proof becomes sound.** It matches variables by identity, not name. Growth through a
   pointer or closure made before the loop defeats a proof. A bound at the counter type's maximum
   (`i <= math.MaxInt`) defeats it too.
2. **Tiger learns four more ways a value shrinks,** each from a loop the trials showed tiger
   rejecting although it ends:
   - Division by a constant greater than 1, or a shift by a constant of at least 1, under a strict
     `> 0` condition: `for size > 0 { size >>= 7 }`.
   - Slicing by the length of a part tiger can show is not empty: `data = data[len(chunk):]`, where
     `chunk` is `data` or `data[:max]` with `max` a constant above 0, and `chunk` is not reassigned
     before the slice.
   - A struct field in the condition, such as `len(q.Items) > 0`, when no call in the body receives
     `q` or a pointer to it.
   - A condition joined with `&&`, where one part shrinking is enough, as in Myers diff's
     `for x > 0 && y > 0 { x--; y-- }`. A negated condition does not count.
3. **A counter counts as a bound only when it is sound.** The comparison is a top-level `&&` part of
   the condition, not inside `||` or `!`. It pairs `<` or `<=` with `++` or `+=`, or `>` or `>=`
   with `--`, never `!=`. Nothing in the body writes the counter or its limit, including through a
   helper, a method, a pointer, or a map insert when the limit is `len(m)`. The limit sits below the
   counter type's maximum.
4. **TS-S02 accepts a loop whose variant tiger verified,** whether tiger found it or a developer
   wrote it. That clears the two-pointer and index-by-2 loops.
5. **A safety cap reports when it is hit, and tiger tells a safety cap from a page size by reading
   the loop.** A counter cap needs no report only when its limit is not a constant and every other
   way out of the loop drains a stream whose type is declared outside the module, such as
   `rows.Next()`, `scanner.Scan()` or `decoder.More()`. That limit is a page size, and reaching it
   means the page is full. Any other way out, an internal condition like `!done()` or a `break` in
   the body, makes the counter a safety cap. It must assert or return an error when the counter
   reaches the limit, wherever the limit comes from: a constant, a parameter, or a field. This
   replaces call 14's "review judges which kind a cap is". On the trial code it fires twice, both
   in git-server's Myers diff: the outer loop needs an assert (Myers guarantees it never reaches
   its limit), and the inner search is a false finding that moving the search into its own function
   clears. The capped worklist becomes the following, which tiger accepts today and whose tests
   show it walking a tree and returning an error on a cycle
   ([`rewrites.go`](experiments/gaps/rewrites.go), [test](experiments/gaps/rewrites_test.go)):

   ```go
   stack := []*Node{root}
   i := 0
   for ; i < len(stack) && i < nodesMax; i++ {
   	stack = append(stack, stack[i].Next...)
   }
   if i == nodesMax {
   	return nil, errWalkTooLarge
   }
   ```

6. **A loop limit has an upper bound where it enters the program.** What matters is where the
   clamp sits, not what it looks like. Tiger traces each loop limit backward: a parameter, field, or
   function result that bounds a loop carries that fact across packages, and every value written
   into it must be a constant, a length, another bounded limit, or a value clamped against a
   declared maximum, such as `min(config.Spins, spinsMax)` or an `if count > objectsMax` that
   rejects. A function that returns a clamped value carries the bound to its callers, so a clamp in
   a validator helper counts. Because the trace starts at the loop, every way a value enters is
   covered: config, flags, requests, and binary formats like a pack header. A call that passes
   `math.MaxInt`, the type's maximum, or a constant at least half of it is flagged too. Calls
   through function values and interface methods are not followed, a recorded miss.

   Deliberation weighed two other designs. A list of input sources (config, flags, environment,
   requests) missed git-server's pack header, the worst bug below. A clamping limit type let the
   clamp sit anywhere and was skipped by JSON or YAML decoding into the field.

   A prototype of this check found three bugs in the trial code that no current rule reports, now
   filed: git-server trusts the object count in a pack header and allocates room for it before
   reading any object, so a 12-byte push claiming 4 million objects allocated 352 MB (ENG-193);
   six of querator's seven list endpoints accept any page size up to 2³¹−1 from the request
   (ENG-194); and querator's create-queue accepts any number of partitions and loops once per
   partition (ENG-195). The tests for changes 5 and 6 are in
   [`cap_reports.go`](experiments/gaps/cap_reports.go), [`config.go`](experiments/gaps/config.go),
   [`boundary.go`](experiments/gaps/boundary.go) and their tests
   ([1](experiments/gaps/cap_reports_test.go), [2](experiments/gaps/boundary_test.go)).
7. **`for ctx.Err() == nil { ... }` counts as a forever loop** when `ctx` is a `context.Context`
   parameter, the same as a `select` on `ctx.Done()`. A context from `context.Background()` or
   `context.TODO()` is never cancelled, so it does not count.
8. **Ranging over an iterator counts as bounded only for the standard library's iterators,** such as
   `slices.All` and `maps.Keys`. Ranging over any other iterator is a TS-S02 finding, like ranging
   over a channel.
9. **Tiger fixes its own 14 loops and TS-S01's documentation in the same change,** using the capped
   form above. The TS-S01 rule reference and its compliant fixture show the capped worklist.
10. **Every loop in `experiments/gaps` becomes a test case in the analyzers,** failing where it runs
   forever and passing once rewritten. Then rerun both trials and record what is left.

Before and after:

| Loop | Today | After |
|---|---|---|
| Sideband writer, `data = data[len(chunk):]` | TS-V01 blocks | passes |
| Varint, `size >>= 7` | TS-V01 blocks | passes |
| Two pointers, `high - low` verified | TS-S02 blocks | passes |
| Refill loop, `len(pending)` with an append | TS-V01 blocks | TS-V01 blocks |
| TS-S01 worklist on a cycle | passes, runs forever | TS-S02 blocks until capped with a report |
| Capped worklist with no report | passes, truncates silently | blocks until it asserts or returns an error |
| The 7 unsound counter loops | pass, run forever | TS-S02 blocks |
| Shadow, pointer, closure growth | pass, run forever | TS-V01 blocks |
| `i < math.MaxInt && !done()` | passes, runs forever | blocks until it reports the cap |
| `SpinLimited(done, limit)` called with `math.MaxInt` | passes, runs forever | blocks until it reports, and the call is flagged |
| A config value passed straight to a loop limit | passes, runs forever | blocks until clamped to a declared maximum |
| `count < limit && scanner.Scan()`, a page over an outside stream | passes | passes, no report needed |
| Range over a non-standard iterator | passes | TS-S02 blocks |
| `for ctx.Err() == nil` on a parameter | both block | passes |
| `for ctx.Err() == nil` on `context.Background()` | both block | both block |

**What remains open.** Three misses are recorded, each with a test that shows the loop running
forever. A limit passed through a function value or an interface method is not traced. A constant below the call-site threshold, such as `1 << 40`, passes as a limit even though
no loop reaches it in practice. And a standard-library stream over an endless source inside the
program, like a `bufio.Scanner` over a reader that never ends, counts as an outside stream, so its
limit needs no report. Both leave a visible trail for review. Ranging over a worklist while
appending to it (`WalkRange`) stays a silent bug no loop rule catches; it ends, so it is a
correctness bug for tests.

**What not agreeing costs.** The 62 findings on correct code stay, and each clears only with a counter cap
like `for budget := len(data); budget > 0 && len(data) > 0; budget-- {`. The 20 loops in
`experiments/gaps` keep passing tiger, and most of them never end. The worklist TS-S01 tells
developers to write keeps running forever on a cycle. The explainer's claim that synthesis "covers
nearly every real loop" stays false on both codebases it has been measured against.

### Call 3: remove the single-implementation interface rule (TS-X01)

**Decision (2026-10-01): Remove.** TS-X01 is removed end to end, with its own ADR.

**Recommendation: Remove.** If we agree, the rule is deleted. If we don't, 24 findings stay, each
with a suggested fix that makes the design worse.

**What the rule enforces.** TS-X01 fires on any interface that has exactly one implementation
outside `_test.go` files, on the theory that such an interface is abstraction added just in case
("we might swap this out someday"). Its message tells the developer to delete the interface and use
the concrete type.

**Where it fires today.** git-server has a ref store, the concrete type that holds branches and
tags. It can read refs, and it can also change them, including through a `CompareAndSwap` method
that atomically moves a ref. The graph API (commit history, diffs, tree walks) only needs to read
refs, so git-server defines a narrow `RefReader` interface with just the read methods and hands the
graph API a `RefReader`, not the ref store:

```go
// RefReader is the ref half of the seam graphapi consumes: listing and resolution
// only. It is deliberately narrower than storage.RefStore — CompareAndSwap is not
// in the surface, so a graph query structurally cannot mutate refs (I2).
type RefReader interface {
	List(ctx context.Context, prefixes []string) ([]storage.Ref, error)
	Resolve(ctx context.Context, name string) (oid storage.OID, ok bool, err error)
}
```

Only one type implements `RefReader`, the ref store, so TS-X01 fires:

```
services/git-server/internal/graphapi/graphapi.go:32:6: TS-X01: interface RefReader has one implementation, memory.refStore — use memory.refStore directly and delete the interface, or add a second implementation outside _test.go files
```

**What following the message would break.** If the graph API took the concrete ref store, it would
have `CompareAndSwap` in reach. Nothing would stop graph-query code, written today or by an agent
next month, from changing refs. The interface is not speculative. Its job is to remove
capabilities, so that "graph queries never change refs" (invariant I2) is enforced by the Go
compiler rather than by convention. The single implementation is the point: the interface narrows
what callers can do.

The rest of the findings are the same kind of seam. The other 14 git-server hits are the storage
ports (`Backend`, `RefStore`, `ObjectStore`, `PackEngine`, `GraphIndex`) that a cross-adapter
conformance suite exists to test, plus the event `Emitter` seam and the `Repos` and `RepoPolicy`
seams between packages. querator's 9 hits are role interfaces like `QueueAdmin` and `UsersAdmin`
over one `service.Service`, where the fix widens every consumer's dependency to the whole service.

**Why removal rather than a fix.** Across both trials TS-X01 fired 24 times, and all 24 were
deliberate seams: narrowing a capability, a test boundary, or a package boundary. None was the
speculative abstraction the rule targets. Tiger cannot tell the two apart from the code's shape.
"One implementation" looks identical whether the interface is pointless or is guarding an
invariant, so every finding pushes toward a fix that makes the design worse, and here it would
weaken a safety boundary. ADR-0006 removes a rule "whose output on real codebases is dominated by
findings a reader would decline to act on, including any finding whose named remedy would break the
code". TS-X01 meets that on both codebases. Whether an interface is the right seam is a design
judgment, which the specification already leaves to review.

**What agreeing changes.** Removal is end to end, meaning the analyzer, its test cases, its
registry entry, and the specification's enforcement line. The maxim stays in the specification with
review as its enforcement. The removal gets its own ADR (see "One ADR per decided call"), recording
the `RefReader` case and the reason above: a shape rule cannot tell a speculative interface from
one that narrows capability, and its remedy damages the second kind.

The first pass of this document proposed an advisory trial instead, pending a git-server rerun.
That rerun is above, and it answered the question the trial would have asked.

**What not agreeing costs.** 24 blocking findings stay whose named fix is a design regression, including
one that would delete a structural safety guarantee. An agent told to reach green follows the
message.

### Call 4: stop requiring a comment on dropped errors in cleanup code (TS-E02)

**Decision (2026-10-01): Keep and change.** TS-E02 stops requiring a comment on an error discarded inside `defer` or `t.Cleanup`. Everywhere else it stays as it is, including the case that found git-server's hash-format bug.

**Recommendation: Change.** If we agree, 47 findings go away. If we don't, a comment is still
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

**What agreeing changes.** A discard inside a `defer` statement's call, or inside a function
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

**What not agreeing costs.** 47 boilerplate comments across two codebases, and the rule keeps teaching agents
that any comment clears it.

### Call 5: don't count a leading `t` or `ctx` toward the four-parameter limit (TS-N07)

**Decision (2026-10-01): Keep and change.** TS-N07 keeps its four-parameter limit and stops counting a leading test handle or `ctx`.

**Recommendation: Change.** If we agree, 21 findings go away. If we don't, the limit keeps
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

**What agreeing changes.** A leading `testing.TB` (or `*testing.T` or `*testing.B`) and a
`context.Context` that comes first or right after it do not count toward the cap. The
adjacent-same-type half is untouched, and that half carries the swap hazards, like the 67
adjacent-`string` findings on git-server.

Before, `assertPartition` counts 5 and fires. After, it counts 3 and passes. Across the two
codebases, 9 of querator's 10 cap findings and 12 of git-server's 21 clear. The ones that remain
are real, like an eight-parameter `Finalize` in querator's benchmarks and an eight-parameter diff
helper in git-server.

**What not agreeing costs.** 21 findings whose fix is an options struct wrapping `name` and `expected`, in helpers
where nobody would swap them. Agents learn to thread `t` through a struct field, which reads worse.

### Call 6: stop checking test files for goroutine-per-item loops (TS-C09)

**Decision (2026-10-01): Keep and change.** TS-C09 stops checking `_test.go` files.

**Recommendation: Change.** If we agree, 12 findings go away and production code is checked as
before. If we don't, test files keep firing.

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

**What agreeing changes.** TS-C09 skips `_test.go` files. Production fan-out still fires.
There were zero production hits on either codebase, so the production behavior is unchanged.

**What not agreeing costs.** 12 findings whose fix breaks the tests they fire in.

### Call 7: make four rules check behavior instead of names (TS-C02, S03, C05, M05)

**Decision (2026-10-01): Keep and change.** All four rules stay blocking. TS-C02 and TS-M05 accept
only exact shapes, TS-S03 and TS-C05 recognize behavior, a new rule requires an event loop's only
exit to be its stop case, and goroutine-leak checks plus shutdown edge tests become the runtime
backstop. Querator's shutdown bugs found along the way are filed as ENG-196.

**Recommendation: Change.** If we agree, renaming no longer passes these rules, correct shutdown
code passes, and the shape every rule message points to is the one Go's own `net/http` uses. If we
don't, the name holes stay open and the rules keep pointing agents at shapes that hide bugs.

**What the rules enforce, and where each trusts a name.**

- **TS-C02**: every `go` statement starts through an owner that waits for it. Today the only
  recognized owner is `errgroup.Group.Go`.
- **TS-S03**: a loop that runs forever selects on a case that can stop it. **TS-C05**: a blocking
  channel wait has a case that can cancel it. Both accept `<-x.Done()` for any `x` with a method
  named `Done`, and any channel whose name contains `shutdown`, `stop`, `quit`, or `done`.
- **TS-M05**: a value put back in a `sync.Pool` is reset first. It accepts any method named
  `Reset`, including an empty one.

**Where they fire, or fail to.** The section 2 fixture passes `tiger check` with a `Done()` that
never fires, a `shutdown` channel nothing closes, and an empty `Reset`. Renaming them gets 5
blocking findings.

An earlier version of this call said querator's daemon was correct `WaitGroup` code that TS-C02
wrongly rejects, and that 10 of querator's 14 TS-C02 findings were that shape. Both were wrong.
`daemon.go` calls `wg.Add` and `wg.Done` but never `wg.Wait`, so `daemon.go:175`, `:209` and `:237`
are the trial bug itself. Of the 14 sites, 4 are correct `WaitGroup` code (`Wait` in `Close`,
`Shutdown`, or the same function), 3 are that bug, and 7 are not `WaitGroup` code at all. The review
also found a hole in today's rule: `wg.Go(f)` or `errgroup.Go(f)` with no `Wait` anywhere passes,
because TS-C02 only looks at `go` statements. That is the trial bug rewritten in a form tiger accepts.

**What the deliberation tested.** Three designs were argued with probes on querator and git-server:

| | Recognize behavior | Strict, exact shapes only | Enforce at runtime |
|---|---|---|---|
| Idea | compute who waits, who closes, what `Reset` clears | accept only `wg.Go`/`errgroup.Go`, `ctx.Done()` on a context, `*b = T{}` | require goleak; drop inference |
| Fakes caught | most; still misses a `close` in dead code and a `Wait` in a method nobody calls | all, by construction | all the leaking ones, when a test runs them |
| Correct code rejected | none of querator's | querator's request-carrying `shutdownCh`, and the best shutdown design below | none |
| Querator's unwaited daemon | caught | caught | **missed 50 of 50 runs**: the goroutine exits a moment after `Shutdown`, before goleak looks |

Then five shutdown designs were built on querator's main and run against the ENG-160 race test and
five edge cases: the caller's ctx expiring while the loop is busy, a retry after that, two
concurrent `Shutdown` calls, a pause queued before shutdown, and a cold request queued before
shutdown.

| Design | Edge cases | Under strict TS-S03 |
|---|---|---|
| main today: handshake on `shutdownCh` plus a flag | fails four; a queued pause or stats call panics the process | rejected |
| handshake, the stop case as the only exit | passes all but the cold request | rejected |
| cancel a stored context, request in an atomic pointer | fails retry, double and cold | accepted |
| `net/http` shape: flag, `close(stop)` once, ctx bounds only the wait | passes all but the cold request | rejected |
| **`net/http` shape, and every waiter also selects on `done`** | **passes all** | rejected |

Strict TS-S03 would reject the best design and accept the worst, so strict is right for TS-C02 and
TS-M05 but wrong for TS-S03 and TS-C05. Go's own guidance points the same way. `net/http`'s
`Server.Shutdown` sets a flag and closes its listeners first, lets the caller's ctx bound only the
wait, and uses the flag only to classify why the loop woke up. Uber's style guide stops a goroutine
by closing a `stop` channel and waits on a `done` channel, with the stop case as the loop's only
`return`. Google's guide and the `context` docs say not to store a context in a struct.

The best design, from querator:

```go
func (l *Logical) Shutdown(ctx context.Context) error {
	l.inShutdown.Store(true)
	l.stopOnce.Do(func() { close(l.stop) })
	select {
	case <-l.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	l.wg.Wait()
	return l.closePartitions(ctx)
}

func (l *Logical) requestLoop() { // started with l.wg.Go
	var state QueueState
	l.prepareQueueState(&state)
	l.serve(&state)          // returns only from `case <-l.stop`
	l.handleShutdown(&state) // answers everything left in requestCh
	close(l.done)
}
```

None of the static designs catch the four querator bugs the edge tests found: an abandoned shutdown
when the caller's ctx expires, a second `Shutdown` returning early, a panic on a queued cold
request, and a cold request never answered. Only running the shutdown paths does. They are filed as
ENG-196 with the redesign and its tests.

**What agreeing changes.** ENG-177 comes back, revised:

| Rule | Today accepts | After, accepts |
|---|---|---|
| TS-C02 | only `errgroup.Group.Go` | only `errgroup.Group.Go` and `sync.WaitGroup.Go`; a bare `go` statement is always a finding. The group's `Wait` must be reached on the success path: in the same function, or for a field, in an exported method of the owning type. A `Shutdown` that returns early on its own ctx expiring is exempt on that path |
| TS-S03, TS-C05 | any `.Done()` | `.Done()` only on a `context.Context`, and not on one that traces within the function to `context.Background()` or `context.TODO()` |
| TS-S03, TS-C05 | any channel named like `shutdown`, or any `chan struct{}` | a channel the package closes or sends on, on the same object: for a struct field, in a method of the owning type reachable from an exported method |
| TS-M05 | any method named `Reset` | only `*b = T{}` before `Put`. The poolzero bug that flags that form after `pool.Get` is fixed first |

Two additions:

- **A new rule: an event loop has one exit, its stop case.** In a TS-S03 loop, no `return` or
  break-out may sit outside the recognized stop case. It is a syntax check, so it is exact. It flags
  the shape behind ENG-160 and passes the best design. It also flags the correct handshake design,
  which pushes agents toward closing a stop channel; its message names the `net/http` shape above as
  the fix. It is measured on querator and git-server before it blocks.
- **A runtime backstop.** Spec rule TS-C03, `goleak` in `TestMain` for every package that starts
  goroutines, is implemented as a shape check: tiger blocks a package with a `go`, `wg.Go` or
  `errgroup.Go` and no goleak harness. TS-S03's message points to shutdown edge tests like the five
  above.

The TS-S03 and TS-C02 messages name one shape as the fix: set a flag, close a stop channel once, let
the loop's only exit be its stop case, have the loop close `done`, start it with `wg.Go`, and have
every waiter select on `done`. The tests for each shape are in
[`experiments/call7`](experiments/call7/).

**What not agreeing costs.** An agent gets past three blocking rules by naming a channel `done` or
writing an empty `Reset`, and past TS-C02 by writing `wg.Go` with no `Wait`. The rules keep pointing
agents at shapes that hid four bugs in querator's shutdown.

### Call 8: accept collect-then-sort in the map-order rule (TS-T02)

**Decision (2026-10-02): Keep and change, plain sorts plus full coverage.** TS-T02 stays blocking
and gains four changes, each proven in `experiments/fullcoverage`:

- **Plain sorts.** A map loop that appends into a local slice passes when the slice's first use
  after the loop is `sort.Strings`, `sort.Ints` or `slices.Sort` on string or integer elements.
  Equal values of those types can't be told apart, so the result never depends on map order. Floats
  are excluded, because -0 and +0 compare equal and print differently.
- **Full coverage.** A comparator sort passes only when its comparisons, taken together, cover every
  field of the element with `cmp.Compare`, `strings.Compare`, or `bytes.Compare` on a whole array.
  Covering every field proves the comparator returns 0 only for identical elements. Pointer,
  interface, bool, float and `time.Time` fields never count as covered. A `Compare` method on a key
  type that passes the same check counts too, which gives struct and array keys a way through:
  `slices.SortedFunc(maps.Keys(m), K.Compare)`. Every other comparator sort fires with its own
  message, which names `slices.Sorted(maps.Keys(m))` followed by `slices.SortStableFunc`.
- **The escape closes.** A `maps.Keys`, `maps.Values` or `maps.All` result fires unless it goes to
  `slices.Sorted`, a full-coverage `SortedFunc`, `maps.Collect` or `maps.Insert`, or a range that
  is checked like a map range. This catches `slices.Collect(maps.Keys(m))`, today's documented miss.
- **No reads of carried state.** A map write gated by a counter, such as copying the first three
  entries visited, fires.

A deliberation weighed plain sorts against trusting every sort and against proving a comparator
total by tracing the key. About 15% of comparator sorts after a map loop in the module cache tie
and leak map order (`docker images`, x/tools linecount, revive, containerd's unmount order,
`go/ast`'s `CommentMap.String`); trusting every sort would pass them silently. Full coverage
replaced the key-tracing proof because it is a check on the element's type, not on what the code
means. The experiment's soundness test runs the prototype's verdict on 72 functions, 9 of them
real sites, and calls each one 200 times; no function it passes prints more than one result, and
every boundary probe it rejects really does vary. On real code full coverage accepts few sites (4 in
970 modules, all in tiger), because most comparators are `sort.Slice` or cover only the key. Its
value is the struct-key path and the closed escape. Plain sorts do most of the work: 91 sites
accepted, including querator's 4. On the trial pins the findings drop from 11 to 7 on querator and
from 17 to 15 on git-server, and none are added. Keys with no order (pointers, interfaces) have no
check tiger can make, so they get a pattern (call 15). Tightening the rest of today's allowlist is a
separate follow-up.

**Recommendation: Change.** If we agree, correct collect-then-sort code stops firing. If we don't,
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

**What agreeing changes.** The analyzer extends its allowlist with those shapes, each backed
by a test case. The ENG-150 blueprint already says the allowlist grows this way, as an
analyzer change and never a config knob. The first shape to add is an append into a local slice
that is sorted before any other use.

Before, the snippet above fires. After, it passes, and the same loop without the `sort.Strings`
still fires.

**What not agreeing costs.** Developers rewrite correct code into the `slices.Sorted(maps.Keys(m))` form. That
is harmless but pure churn. The rule is not wrong. It is narrower than it needs to be.

### Call 9: add a baseline file that grandfathers existing findings

**Decision (2026-10-02): Reject.** Tiger gets no baseline mechanism and does not recommend one.
`config/golangci.yml` drops its line telling adopters to use `--new-from-rev=origin/main`. An
adopter who chooses golangci-lint's own new-code filters (`--new-from-rev`, `new-from-merge-base`,
`new-from-patch`, `issues.new`) may use them, and tiger does not check for them;
that is the adopter's call, not tiger's default. `experiments/newfromrev` shows what the filter
does to tiger's findings: with `--new-from-rev=HEAD~1` the plugin drops the finding on the line the
branch didn't touch and prints nothing about it, with `--new-from-rev=HEAD` a repository with two
blocking findings passes, and run through a symlinked path it drops every finding, including the
new one. `tiger check` reads no git history and reports both.

**Recommendation: Reject.** If we agree, nothing changes. If we don't, tiger gets a mechanism
that records today's findings and fails only on new ones.

**What it governs.** Some linters let an adopter record today's findings in a file and fail only
on new ones. Tiger has no such file for blocking rules.

**Where it bites.** A first run prints 396 rule findings on querator and 323 on git-server. An
existing codebase cannot adopt tiger incrementally. It has to fix everything or disable
analyzers.

**What agreeing changes.** Nothing. This records the cost instead of paying it with a
mechanism. ADR-0011 refused the same mechanism for advisory findings, because a file that raises
a limit is the cheapest path to green an agent can find. The argument is the same for blocking
findings. Tiger's stated target is new AI-written code, where this cost does not arise.

**What not agreeing costs.** A baseline lets brownfield teams adopt tiger in a day. It also gives an agent a
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

**Decision (2026-10-02): Keep and change, as an opt-in.** Whether a codebase uses `//nolint` is
the adopter's choice, so tiger does not ban it by default. A `tiger.yaml` entry turns the ban on:

```yaml
nolint:
  forbid: true
  reason: agents must fix findings, not silence them
```

With it on, every `//nolint` in a checked package is a blocking TS-L09 finding, reported at the
file's `package` line. `tiger check` enforces it. Under the golangci-lint plugin it can be bypassed,
and the docs say so: `experiments/nolint` shows a file-wide `//nolint:tiger` in the package doc
comment dropping even a finding at the `package` line, and an `exclusions` rule in
`.golangci.yml` dropping findings without touching the code. `tiger check` never honors
`//nolint`. A bare `//nolint` does satisfy TS-E02's "say why in a comment", but only because the
rule checks that a comment exists, which `// x` also satisfies (call 4), so rejecting `//nolint`
there would close nothing. The explainer drops "tiger flags any `//nolint`", and the
specification's TS-L09 line describes the opt-in. The counted `//nolint:<linter>` waiver proposed
below is not adopted.

**Recommendation: Change.** If we agree, the silent bypass closes and `//nolint` for other
linters is counted. If we don't, `//nolint` keeps silencing tiger's rules.

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

**What agreeing changes.** Tiger's own rules accept no suppression, whether tiger runs as
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

**What not agreeing costs.** Under the plugin, one comment silences any blocking rule with no trace. Under the
CLI, a `//nolint` clears any rule that asks for a comment, by accident. The explainer keeps promising a ban
that does not exist. The alternative, banning `//nolint` for every enabled linter, is defensible
and simpler to explain. It was not chosen because it forces adopters to turn off whole linters
over one false positive, which is louder but loses more signal than a counted waiver does.

### Call 11: keep `//tiger:restrict` opt-in and restore it in the explainer (TS-P01, P02, K03)

**Decision (2026-10-04): Remove all package-level declarations.** `//tiger:restrict` goes, with
TS-P01, P02, P03 and K03 and the rule that requires pins on every exported function of a
closed-dispatch package. Each axis is either redundant or unworkable. `no-reflect` repeats TS-S12,
which already bans `reflect` everywhere. `imports(...)` repeats TS-D01's default and the TS-X03
layer file. `closed-dispatch` is not from TigerStyle or Power of Ten (the nearest ancestor is Power
of Ten's ban on function pointers), no codebase declares it, and it contradicts tiger's own
cancellation rules: a probe that waits the way TS-C05 requires gets two TS-K03 findings on
`ctx.Done()` and `ctx.Err()`, whose named fix is impossible because no concrete type exists for a
`context.Context` you are handed. Pins stay opt-in, as the documentation describes; what replaces
effect and frame pins is decided separately.

**Recommendation: Keep.** If we agree, the tool stays as it is and the explainer gets the
directive back. If we don't, restrictions turn on for every package by default.

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

**What agreeing changes.** The shipped design stays. The restriction set is an opt-in claim a
package makes, and absence is never a finding. The explainer restores the paragraph that
introduces the directive. Without it, a reader who meets TS-P01 in the rule reference has no way
to learn what a restriction set is. The explainer was half right about one axis. Reflection is
already banned everywhere by the golangci-lint linter behind TS-S12, and `no-reflect` on the directive only
makes that claim checkable by dependents. The explainer should say so.

**What not agreeing costs.** Either the rule reference documents three blocking rules the explainer never
mentions, or restriction becomes default-on and every adopter meets findings like the 68 above on
day one.

### Call 12: fix the stale map-order wording in the spec and explainer (TS-T02)

**Decision (2026-10-04): Fix docs, describing the rule as call 8 changes it.** The documents
describe the decided rule, not today's analyzer, and drop "exact" as well as "heuristic".

- **Explainer.** TS-T02 bans every map range except an allowlist of shapes that can't depend on
  order. The allowlist includes collect-then-sort with a plain sort, or with a comparator that
  covers every field (call 8). A finding means the loop's shape is outside the allowlist, not that
  the output really varies; 72% of the rejected comparator sorts in call 8's sample can't tie. The
  rule passes five shapes that do depend on order, and the double-run test (TS-T11) backs them up.
- **Specification.** The TS-T02 enforcement line and the Part V analyzer table describe the
  allowlist, the closed `maps.Keys`/`Values`/`All` escape, and the known misses, in place of
  "range over a map whose body appends or writes. Heuristic".
- **Known misses**, from `experiments/fullcoverage`: last writer wins (`inverse[v] = k` when two
  keys share a value), a map write whose value calls a function (`labels[k] = fmt.Sprint(v)`),
  `golang.org/x/exp/maps.Keys`, `reflect.Value.MapKeys`, and a range over a type parameter
  constrained to a map.

The proposal below was written before call 8. The escape it names (collect the keys, never sort,
range the slice) is closed by call 8, and "exact over a conservative allowlist" overstates a rule
with known misses.

**Recommendation: Fix docs.** If we agree, the specification and explainer are corrected and the
tool is unchanged. If we don't, the documents stay inconsistent with each other and the code.

**What the three sources say.** The explainer says the map-order check bans every map range except
a fixed set of safe body shapes, and calls it "a heuristic rather than a proof". The
specification's analyzer table says the analyzer flags a "range over a map whose body appends or
writes. Heuristic". The code does what the explainer describes, the allowlist inversion shown in
call 8. The ENG-150 blueprint made that inversion on purpose, called it exact, and flagged the
specification line for amendment. The amendment never landed.

**What agreeing changes.** Nothing in the product changes. This is a disagreement between documents only.

- The specification's TS-T02 enforcement line and its Part V analyzer table change to describe
  the allowlist.
- The explainer drops "heuristic". When the rule fires, the loop really does have a shape outside
  the safe list; it never guesses. It is not a proof that order never reaches an output, because
  one shape it deliberately ignores (collect the keys into a slice, never sort, then range the
  slice) gets past it. The replacement wording is "exact over a
  conservative allowlist, backstopped by the double-run test (TS-T11)".

**What not agreeing costs.** The specification keeps describing an analyzer tiger does not ship, and the
explainer keeps calling a rule that never guesses a heuristic, which invites readers to treat its findings as
optional.

---

## 5. Loop bounds, revisited

Calls 1 and 2 raised a question the first version did not answer: what can tiger actually prove
about a database loop like `for rows.Next()`? This section answers it from querator's 30
cursor-shaped loops and two experiments committed under [`experiments/`](experiments/).

### What querator's 30 loops actually do

These are the 30 loops where TS-S02 and TS-V01 both fire on querator.

| Group | Loops | Examples | Where the limit lives |
|---|---|---|---|
| Store reads, limit in the query | 8 | Postgres `List`, `ListScheduled`, `Lease`; Mongo `List` | `LIMIT $2` in the SQL, or `SetLimit(opts.Limit)` in Mongo |
| Store reads, limit in the loop body | 9 | Badger `Lease` and the eight `List` functions | `if count >= opts.Limit { return nil }` |
| Whole-partition scans | 5 | Badger `Clear`, `Stats`, `ScanForActions` | none; all five also name their context `_`, so they cannot be cancelled |
| Not store loops | 8 | 4 tests, a benchmark, a file reader, the shutdown drain, the batch iterator | mixed: test timeouts, `b.N`, other goroutines, a counter |

Seventeen loops already declare a limit. Tiger cannot see it, because it sits in SQL text or in a
check deep in the loop body.

There are two separate bounds. One is how many items the loop produces, which governs memory and is
the out-of-memory failure B1 warns about. The other is how many times the loop runs, which governs
time. Six of the nine Badger loops skip rows before counting them:

```go
if namespace != "" && role.Namespace != namespace {
	continue          // skipped rows don't count
}
if count >= opts.Limit {
	return nil
}
*roles = append(*roles, role)
count++
```

Tiger can prove this never returns more than `opts.Limit` roles. It cannot prove the loop runs at
most `opts.Limit` times. A namespace with one role in a table of a million still walks the million.

### Experiment 1: a restated limit bounds a cursor loop

[`experiments/cursorbound`](experiments/cursorbound/) writes the same list loop two ways and runs
it through `tiger check` and `go test`. A fake cursor stands in for the database and counts every
`Next()` call ([test](experiments/cursorbound/capped_test.go),
[test output](experiments/cursorbound/test.out), [tiger output](experiments/cursorbound/tiger.out)).

```go
func ListUncapped(rows Rows) []string {
	var names []string
	for rows.Next() {
		names = append(names, rows.Value())
	}
	return names
}

func ListCapped(rows Rows, limit int) []string {
	var names []string
	for count := 0; count < limit && rows.Next(); count++ {
		names = append(names, rows.Value())
	}
	return names
}
```

`tiger check` flags only the uncapped loop:

```
uncapped.go:7:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
uncapped.go:7:6: TS-S02: tiger can't tell how many times this loop runs: its condition is not a constant, a len, or a counter — add a cap (for tries := 0; tries < max; tries++) that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
tiger: 2 blocking
```

All five runtime cases pass:

| Case | Database LIMIT | Loop limit | Rows returned | `Next()` calls |
|---|---|---|---|---|
| Database stops first | 50 | 100 | 50 | 51 |
| Limits agree | 100 | 100 | 100 | 100 |
| Loop stops first | 100 | 50 | 50 (truncated) | 50 |
| Database ignores LIMIT | none | 100 | 100 | 100 |
| Uncapped loop | 1,000,000 | none | 1,000,000 | 1,000,001 |

The restated limit holds whatever the database does, even when a driver ignores `LIMIT` and sends
rows forever. When the loop's limit is smaller than the query's, rows get dropped. That is a
correctness bug for tests and review, not a loop that never ends. Tiger cannot check that the two
limits agree, because that would mean parsing SQL and knowing each driver's API.

### Experiment 2: how tiger checks an annotation

Two annotations deal with loop bounds, and tiger treats them in opposite ways. It proves a
`//tiger:variant` against the loop body and blocks the build if the proof fails. It never reads a
`//tiger:batched` reason, so a false reason passes just like a true one.
[`experiments/annotations`](experiments/annotations/) puts each annotation on one loop where it is
true and one where it is false, runs every loop through `tiger check`, and has `go test` run each
loop to see whether it ends. A loop still running after 300 ms counts as running forever
([test](experiments/annotations/annotations_test.go),
[test output](experiments/annotations/test.out), [tiger output](experiments/annotations/tiger.out)).

**`//tiger:variant` is a claim tiger proves.** A variant names a number that gets smaller on every
pass and keeps the loop running only while it is above zero, so the loop has to end. Tiger accepts
the claim only after three checks:

1. The expression fits the grammar: integer literals, variables, and `len(x)`, joined by `+` and `-`.
2. The loop's condition compares that expression with a fixed value, as in `len(pending) > 0`,
   `i > 0`, or `low < high`.
3. The body shrinks it on every pass with a move tiger knows (`s = s[1:]`, `i--`, `i += 2`), and no
   path grows it back or skips the shrink with `continue`.

The true claim passes with no findings, and the test confirms the loop ends:

```go
//tiger:variant len(pending)
for len(pending) > 0 {
	pending = pending[1:]
	drained++
}
```

The same claim on a body that appends back into `pending` is blocked. Fed a `refill` that always
returns one item, the loop never ends:

```go
//tiger:variant len(pending)
for len(pending) > 0 {
	pending = pending[1:]
	drained++
	pending = append(pending, refill()...)
}
```

```
variant_lie_refill.go:8:2: TS-V01: //tiger:variant len(pending) doesn't provably shrink on every pass (len(pending) cannot be shown to move consistently) — add a cap (for tries := 0; tries < max; tries++) that fails when hit, or name an expression that does shrink
```

A `continue` before the shrink is blocked the same way, and that loop also runs forever:

```
variant_lie_skip.go:8:2: TS-V01: //tiger:variant len(pending) doesn't provably shrink on every pass (a continue can skip the loop's decrease) — add a cap (for tries := 0; tries < max; tries++) that fails when hit, or name an expression that does shrink
```

A cursor loop has no honest variant to write. `rows.Next()` is not a comparison, so tiger rejects
any expression placed on it:

```
variant_cursor.go:8:2: TS-V01: //tiger:variant remaining doesn't provably shrink on every pass (the loop's condition is not one of the recognized comparison forms) — add a cap (for tries := 0; tries < max; tries++) that fails when hit, or name an expression that does shrink
```

**`//tiger:batched` is a claim tiger takes on trust.** Tiger checks only that the loop's condition
is a method call on a plain variable, like `rows.Next()` or `it.Valid()`, and that some text
follows the annotation. When both hold, TS-S02 stays quiet. These two loops carry the same reason.
The first one's cursor returns 50 rows. The second one's never runs out:

```go
// batched_honest.go
//tiger:batched rows come from SELECT ... LIMIT $1
for rows.Next() {
	names = append(names, rows.Value())
}

// batched_lie.go
//tiger:batched rows come from SELECT ... LIMIT $1
for rows.Next() {
	count++
}
```

Tiger reports the same thing for both, and neither gets a TS-S02 finding:

```
batched_honest.go:7:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — …
batched_lie.go:9:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — …
```

The honest loop returns 50 rows. The other one never returns. The TS-V01 finding on both is the
clash tracked in ENG-183, where one rule honors the waiver and the other does not. The shape check
does stop the annotation on a loop that is not cursor-shaped, like `for !ready()`, and tiger blocks
an annotation with no reason after it. Neither check looks at whether the reason is true.

| Loop | Annotation | Tiger reports | At runtime |
|---|---|---|---|
| Drain | `variant len(pending)`, true | nothing | ends |
| Drain with refill | same, false | TS-V01 blocks | runs forever |
| Drain with skip | same, false | TS-V01 blocks | runs forever |
| List, 50 rows | `batched`, true | TS-V01 only | ends |
| Count, endless rows | `batched`, false | TS-V01 only, same as above | runs forever |
| Restated limit, endless rows | none | nothing | ends after 100 rows |

Tiger catches every false variant in this experiment and none of the false batched reasons. The
restated limit needs no annotation, because the proof is the counter in the loop header. That is
why call 13 drops `//tiger:batched` as a loop-bound waiver instead of trying to make tiger check its
reason.

### Call 13: stop using `//tiger:batched` to waive the loop bound (TS-S02, ADR-0004)

**Recommendation: Change.** If we agree, store loops restate the limit they already declare,
whole-partition scans get a declared maximum or cancellation, and `//tiger:batched` keeps its other
job. If we don't, the waiver keeps standing in for a limit nobody declared.

**What the rule enforces.** TS-S02 requires every loop to state a bound tiger can see. ADR-0004 lets
`//tiger:batched <reason>` waive that on a cursor-shaped loop, reasoning that "the store is finite"
and that any cap would be "either a magic number or a restatement of however big the store is".
The same directive also waives TS-M10, the rule against IO inside a loop.

**Where it applies today.** The audit above shows ADR-0004's premise does not hold for most of
these loops. Seventeen declare a real limit, the page or request size, so restating it is no magic
number. B1 also rejects "the store is finite" as a bound. A finite store with no declared limit is
exactly the limit "chosen by whatever runs out first". ADR-0004 also clashes with TS-V01, which the
spec already records as open item ENG-183.

**What agreeing changes.** `//tiger:batched` stops waiving TS-S02 and stays as the waiver for
TS-M10. A new ADR replaces ADR-0004's loop-bound half and closes ENG-183.

Before:

```go
//tiger:batched rows arrive from a Postgres cursor; the table size is the bound
for rows.Next() {
```

After:

```go
for count := 0; count < opts.Limit && rows.Next(); count++ {
```

The five whole-partition scans need one of the two shapes B1 allows. One is a declared maximum
partition size, with the loop capped at it and an assert if it is exceeded. The other is paging
that checks `ctx` on every pass so the scan can be cancelled.

**What not agreeing costs.** A reason string keeps standing in for a declared limit. Loops that already
have a limit carry a waiver they do not need. The five scans that cannot be cancelled stay hidden
behind "the table size is the bound", and the clash with TS-V01 stays open.

**Open question, not decided here.** Do filtered loops need a bound on how many times they run, or
is a bound on how many items they return enough? With the restated limit, `count` counts every row
examined, so a namespace filter can return fewer than `opts.Limit` matches even when more exist.
Counting only matches keeps the results right but leaves the run count bounded only by the table's
size. That is a product decision about list semantics, so it is not a call here.

### Call 14: tell a safety cap from a page size by the loop's other exit (TS-S02)

**Recommendation: Change.** If we agree, tiger decides which caps must report by reading the loop,
and the spec and message say so. If we don't, the spec keeps asking for an assert on every cap while
the tool accepts caps that stop silently, and review judges which kind each cap is.

An earlier version recommended only a docs fix, with review telling the two kinds apart. A
deliberation on call 2's cap rules found a deterministic way to tell them apart, so this call now
changes the tool.

**What the three sources say.** The spec says the compliant form is "an explicit iteration cap with
an assert on exhaustion". Tiger's message says to add a cap "that fails when the cap is hit". But
tiger accepted `ListCapped` in experiment 1 with no assert at all.

**The two kinds of cap.** A *safety cap* is a limit the code should never reach, such as the maximum
depth of a delta chain. Reaching it means something is wrong, so it should assert or return an
error. A *page size* is part of the behavior, such as the size of a list page. Reaching it is
normal, because the page is full, and asserting there would fail every full page.

**What agreeing changes.** Tiger reads the loop's other way out (call 2, change 5). When every other
exit drains a stream from outside the module, like `rows.Next()`, the counter is a page size and
needs no report. When the other exit is an internal condition or a `break`, the counter is a safety
cap and must assert or return an error when reached. The spec line and the message state that rule,
and both drop "review judges which kind a cap is". Every loop limit also needs an upper bound where
it enters the program (change 6), so a page size read from a request is clamped to a declared
maximum.

**What not agreeing costs.** The spec and the tool keep disagreeing, and a cap that stops silently on a
cycle or a stuck condition keeps passing, as `WalkCapped` and `SpinLimited` show in
`experiments/gaps`.

---

## 6. What the rules can't check

Calls 2 and 7 found correct shapes that tiger can check only in part. This section adds a second
part to tiger for the rest.

### Call 15: ship a tested patterns collection for what the rules can't check

**Decision (2026-10-01): Agreed, with patterns as packages in the repository and built into the
binary.** Patterns live as tested packages under `patterns/`, the `tiger` binary embeds them and
prints them with `tiger pattern <name>`, and every finding from a rule a pattern backs names that
pattern. A skill can be generated later from the same files if reviewer agents need one.

**Recommendation: Change.** If we agree, tiger gains a second part next to its rules: a small
collection of runnable patterns, each covering a gap where a rule stops short and a real bug lived.
Rule messages point to them, and agent reviewers check diffs against them. If we don't, the parts
of a correct shape that no rule can check stay in review with nothing concrete to compare against,
and agents keep improvising them.

**Why the rules alone are not enough.** Calls 2 and 7 found shapes tiger can check only in part.
The shutdown loop is the clearest case. Tiger can check how the loop goroutine starts, that its
only exit is its stop case, and that the stop case is a real context or a channel the package
closes. It cannot check that the loop runs cleanup before it closes `done`, that `Shutdown` lets
the caller's context bound only its wait, or that every caller waiting on the loop's reply also
selects on `done`. Querator's shutdown bugs (ENG-160, ENG-196) lived in exactly those parts.

**The precedent.** `examples/ledger` is already a worked pattern: the invariant vocabulary, with an
`inv` package, an encode/decode pair asserting it, and a violation test per invariant. The README
points to it and the Explainer walks through it. The specification's "How review divides" section
already assigns some questions to reviewers. This call makes that a deliberate part of tiger.

**The entry condition.** A pattern exists only where a tiger rule stops short and a real bug lived
in that gap: a trial finding, a filed ticket, or a bug an experiment reproduced. A shape that is
merely good Go, with no rule it backs up and no bug behind it, does not qualify. This keeps the
collection from becoming a general Go style guide.

**What makes a pattern more than a wiki that goes stale.** Every pattern meets all five
requirements below. They are recorded in the repository's `CLAUDE.md` so that agents adding rules or
patterns follow them.

1. **Each pattern is runnable code, not prose.** A small Go package showing the right shape, next to
   the broken shape it replaces. Tests demonstrate both, the way `experiments/call7` shows
   `StopDoneLoop` cleaning up every time and `EarlyExitLoop` skipping cleanup. Tests are the proof.
   The broken shape shows only the gap: it passes `tiger check`, and its test shows the bug.
2. **CI keeps the patterns honest.** Every pattern must pass `tiger check` with zero findings and run
   its tests under `-race`. A pattern can't fall out of date with the rules, because the build fails
   first.
3. **Every pattern names the rules it backs up and the part those rules can't see.** For example,
   the shutdown pattern backs TS-C02, TS-S03 and TS-C05. Tiger can check how the loop starts and
   stops, but not whether every waiter also watches `done`. That missing check is exactly what the
   pattern teaches.
4. **Rule messages point to the pattern.** TS-S03's message ends with something like "see pattern
   `shutdown-loop`", so an agent that hits the finding reads the right shape in place of
   improvising one.
5. **Each pattern gives reviewers a short checklist.** It covers the parts tiger can't check, as
   yes-or-no questions an agent reviewer can answer while comparing a diff to the pattern. For the
   shutdown loop: does every caller waiting on the loop's reply also select on `done`? Does
   `Shutdown` close stop before it waits? Is there a test that calls `Shutdown` with an expired
   context?

**The seed patterns.** Seven already exist as tested code from this decision's experiments:

| Pattern | Comes from | Backs up | The bug behind it |
|---|---|---|---|
| Shutdown loop: stop and done, one exit | `call7/StopDoneLoop` | TS-C02, S03, C05 | ENG-160, ENG-196 |
| Goroutine owner: `wg.Go` and `Wait` | `call7/WaitedDaemon` | TS-C02 | querator's unwaited daemon (ENG-148 trial) |
| Pool reuse: zero before `Put` | `call7/UseWithZero` | TS-M05 | the empty `Reset` fixture leaking the last user |
| Capped worklist that reports when the cap is hit | `gaps/WalkReported` | TS-S02, S01 | git-server's tag-peel denial of service (ENG-159 trial) |
| Page over an outside stream | `gaps/ReadLines`, `cursorbound` | TS-S02, call 13 | querator's whole-partition scans |
| Clamp a limit where it enters the program | `gaps/ListClamped`, `gaps/ParsePack` | call 2, change 6 | ENG-193, ENG-194, ENG-195 |
| Sorted map output when the key has no order | `fullcoverage/cases/checker`, `cases/commentmap` | TS-T02 (call 8) | Go issues #27013 and #30202, `go/ast`'s `CommentMap.String` |

`examples/ledger` joins them as the invariant pattern, which backs TS-A07, A08 and A09.

**What is not part of this call.** A reusable runtime package that encodes these shapes, such as a
`lifecycle` package for the shutdown loop, was considered and set aside. Most of these shapes don't
reduce to a reusable package, and a package that adds code overall is not clearly better than
following the pattern. Following the patterns is the goal.

**The format.** A deliberation weighed three formats against the five requirements:

| | Packages in the repository only | Packages plus the binary | Packages plus a generated agent skill |
|---|---|---|---|
| How an agent gets the pattern | the finding names a slug; the agent finds the tiger module or a GitHub URL at the right tag | the finding says `see \`tiger pattern shutdown-loop\``; running it prints the card, the code and the tests | the skill's description matches the finding and the agent loads it |
| Strongest argument | nothing new to build; `examples/ledger` already works this way | the pattern always comes from the same release as the finding, the pointer is checked by a test, and it works offline with no install | gives reviewers a procedure and can load before a finding exists |
| Biggest risk | nothing in the finding delivers the code | adopters on the golangci plugin alone have no `tiger` command | the model decides whether a skill loads; it needs an install in every repo |

Every option starts from the same source, tested packages in tiger's root module, so the question
was what to add on top. Adopters don't have tiger's source: querator does not depend on the tiger
module at all, and copies `assert/` in. The binary is the only delivery where the pattern an agent
reads is guaranteed to match the rules that flagged the code, and drift between the two is how a
pattern collection goes stale.

**What agreeing changes.**

- Patterns live under `patterns/<name>/` in tiger's root module, so the existing dogfood job and
  `go test ./...` cover them. `examples/` is renamed `patterns/`, with `examples/ledger` and the seven
  seed patterns as the first entries. Each seed is cleaned up to meet the requirements before it is
  added; `experiments/call7` gets 28 findings today, most from its test helper and file layout.
- The `tiger` binary embeds the patterns. `tiger pattern <name>` prints a pattern's card (the rules
  it backs, the gap, the bug behind it, the reviewer checklist), its code and its tests.
  `tiger pattern --rule <code>` lists the patterns behind a rule, and `--checklist` prints only the
  reviewer questions.
- Each rule's registry entry names the patterns it is backed by, and the driver appends the pointer
  (`see \`tiger pattern shutdown-loop\``) to every finding from that rule, so a rule violation
  leads the agent to the correct shape. The analyzers do not change. A meta-test fails if a pointer
  names a pattern that does not exist, or if a pattern names a rule that does not exist, and the
  message length cap counts the pointer.
- Releases ship the `tiger` binary alongside the golangci plugin, so plugin-only adopters can run
  `tiger pattern`. An adopter's agent instructions gain one line: findings that name a pattern, run
  `tiger pattern <name>`.
- A skill for reviewer agents is not built now. If one is needed, it is generated from the same
  embedded files.
- CI runs `tiger check` and the tests under `-race` on every pattern, and fails on any finding.
- The messages of the rules each pattern backs up gain a pointer to it.
- Every pattern carries its rules, its gap, the bug behind it, and its reviewer checklist.
- The repository's `CLAUDE.md` records the entry condition and the five requirements.
- The specification's "How review divides" section points reviewers at the checklists.

**What not agreeing costs.** The parts of a correct shape no rule can check stay with review, and
review has nothing concrete to compare a diff against. An agent fixing a TS-S03 finding improvises
a shutdown loop, and querator shows what improvised shutdown loops look like.

### Call 16: replace effect and frame pins with a `tiger changed` report (TS-F01, F02, F07)

**Decision (2026-10-04): Keep and change; the report replaces pins.** `//tiger:effects` and
`//tiger:frame` pins go, with TS-F01, F02 and F07, the effect table and the effects and frames
analyzers. `tiger changed <base>` replaces them: for each struct (or free function), it lists the
types and functions it calls now that it did not call at the base revision. A new method on a type
the struct already calls is not reported. Nothing is annotated and nothing is propagated to
callers. It is a report for review, human or agent, not a rule, so ADR-0012 does not apply.
`//tiger:variant` stays; `//tiger:requires` and `//tiger:ensures` are not part of this call. The
report's output format is a follow-up ticket.

**What pins were for.** The explainer's own example is the goal: if the architecture allows
database access only through a `Store` interface and an agent adds a database call somewhere else,
the reviewer should learn that the PR changed the architecture, not only the code. That goal stays.

**Why pins could not meet it.** A probe showed tiger computing `func Save(s Storage, e string)
error { return s.Write(e) }` as having no effects and accepting `//tiger:effects none` on it, while
a test passed a `Disk` and a file was written. On querator, all 276 functions on the request path,
from the HTTP handlers to the Postgres, Mongo and Badger stores, compute as doing no IO, because
tiger drops calls through interfaces and calls into third-party libraries. `fmt.Fprintf` counts as
disk IO, so a pure SHA-1 hash fails a pin. Frames miss a write through a value receiver's pointer
field and a map `delete` through the receiver. Fixing effects needs either a table mapping every
library's methods to effects, a maintenance burden on tiger and adopters that never covers pgx or
the next driver, or closed dispatch, which call 11 removed. Pinning an interface is also the wrong
question: `Save` does disk IO with a `Disk` and none with a test double, so only concrete code has
one true answer. And a pin is a comment the agent edits in the same commit as the code; it helps
only when a change three calls deep forces an edit to a pin the human cares about, and then only
if someone notices the pin line changed.

**What the deliberation weighed.** Three advocates prototyped on querator and git-server. Reach
pins (a pin listing the outside code a function calls, by Go name) caught 23 real behavior changes
over 10 querator commits with no table, but pins high in the call stack ran to about 1,300
characters and the agent still edits them. Effects through the signature (IO only through values
passed in, locked by a `//tiger:uses` pin) caught 4 of 6 IO changes but needed a package-level
declaration call 11 had just removed. Removing pins cost nothing in use (no codebase has one) but
left the goal unmet. The owner's question settled it: if the goal is a report of what changed
between the base branch and the PR, tiger can compute both sides itself and needs no pins.

**What the report shows, and where.** A change is reported on the struct where it was written,
not on everything that calls it: if B starts calling the database and A calls B, only B is
reported, because B is where the surprising change happened. Work handed over a channel shows on
the struct at the receiving end when it makes the new call. Calls are named by what the code
calls: an interface call is named by the interface, never by a guess at the implementation, so
the report needs no table. Pure helpers are kept out by a filter computed from the import graph,
with no list of packages: a method or interface counts when its package reaches `syscall`, and a
free function counts when its own body does, which keeps `os`, `net`, `time`, pgx and badger and
drops `strings`, `sort` and `fmt`. Generated files are skipped and `context.Context` is shown.

**The evidence.** `experiments/changed` holds the prototype (537 lines) and 29 tests on a fixture
git repository, including a handler that starts calling the database, reach not propagated to
callers, an interface call named as the interface, a new method on a known type staying silent, a
new constructor reported, and the receiving end of a channel reported. On 10 querator commits it
printed 5 lines, 2 of them real: commit aeb014c, where the daemon starts constructing the Postgres
stores. On git-server's head commit it printed 173 lines, 110 from generated protobuf code; the
mixed filter and skipping generated files bring that to 36 hand-written lines.

**Known blind spots,** each pinned by a test: a write through `io.Writer` is silent, because the
`io` package never reaches the OS; moving a call into a separate free function reads as one edge
removed and one added; a renamed type reads as all its edges removed and re-added. Whether to add
a section for new methods on known types is an open question for the output-format ticket.

---

## What happens next

### One ADR per decided call

Once the calls here are decided, each one gets its own ADR (architecture decision record) in
`docs/adr/` before any implementation work starts. The ADR records what was decided and why, and
cites this document and its experiments as the context. A call we don't agree to gets an ADR too, so the
reason for leaving a rule alone is on record. Where a decision replaces part of an existing ADR,
as call 13 does to ADR-0004, the new ADR says so and the old one is marked superseded in part.

### The canceled tickets

Eleven backlog tickets were canceled while this question was open. The direction holds, so most of
them describe work this decision still wants. Reopening is for whoever decides these calls. Nothing here
reopens a ticket.

| Ticket | Disposition |
|---|---|
| ENG-177, recognize behavior, not names | revive as written (call 7) |
| ENG-178, close uncounted silencing channels | revive items 3 and 4: recognize `package assert` by import path, drop the unread `hot`/`wire`/`owner` verbs. Drop item 1; one advisory per generated file is noise of the kind ADR-0006 removes. Drop item 2; call 10 leaves `//nolint` under the plugin to the adopter |
| ENG-179, trusted declarations get the escape treatment | revive item 1 (`//tiger:openenum` counted as an escape with a reason). Drop item 2; call 4's cleanup exemption replaces the proposed `//tiger:discard` directive |
| ENG-176, revive naming rules on the config file | leave canceled; scoping was never the failure |
| ENG-175, exemptions for signatures pinned by third-party interfaces | leave canceled; one site in one repo, and an adapter function is the compliant shape |
| ENG-188, reconcile spec with explainer | superseded by this document and its follow-ups |
| ENG-181, specification gaps | fold into the specification follow-up below |
| ENG-174, JSON output | revive when convenient; both trial reports asked for it |
| ENG-171, 172, 173, auto-fix, editor, dashboards | leave canceled until the rule set above settles |

### Follow-ups, in order

1. **Make both loop rules sound, then teach TS-V01 more proofs** (call 2), in one release. Fix
   TS-V01's name matching, outside-the-body growth and wraparound; tighten TS-S02's counter check;
   require a safety cap to report when hit, told apart from a page size by the loop's other exit; require every loop limit to be bounded where it enters the program; accept a verified variant, a `ctx.Err()` loop on a
   context parameter, and standard-library iterators; add the four shrink forms. Every loop in
   `experiments/gaps`, `experiments/variants` and `experiments/annotations` becomes an analyzer test
   case. Build change 6 starting from [`experiments/limitfacts`](experiments/limitfacts/), adding
   what its README lists as missing. Fix tiger's own 14 worklist loops and TS-S01's rule reference
   and fixture. Rerun both codebases and record what is left.
2. **ENG-177, revised** (call 7). TS-C02 and TS-M05 accept only exact shapes, with TS-C02's
   `Wait` check; TS-S03 and TS-C05 recognize behavior; fix poolzero's false finding on `*b = T{}`
   after `pool.Get`; add the one-exit rule for event loops, measured on both trials first; implement
   TS-C03's goleak harness check; point every message at the stop-and-done shape. Each shape in
   `experiments/call7` becomes an analyzer test case. Rerun both codebases.
3. **Remove TS-X01** (call 3), end to end per ADR-0006, with an ADR recording why: a shape rule
   cannot tell a speculative interface from one that narrows capability, like git-server's
   `RefReader`, and its remedy damages the second kind. Remove `//tiger:restrict` (call 11) the same
   way: TS-P01, P02, P03 and K03, the `restrictions` and `closedworld` analyzers, the directive, and
   the closed-dispatch pin requirement.
4. **Exemptions for TS-E02, TS-N07, TS-C09** (calls 4, 5, 6), each with a test case for the
   exempted shape.
5. **Silencing channels** (call 10). ENG-178 items 3 and 4, ENG-179 item 1, and the opt-in
   `nolint.forbid` entry in `tiger.yaml`, enforced by `tiger check`.
   Drop the `--new-from-rev` recommendation from `config/golangci.yml` in the same change (call 9).
6. **TS-T02 collect-then-sort, full coverage and the closed escape** (call 8). Build from
   `experiments/fullcoverage`, keeping its soundness test. A second ticket tightens the rest of the
   allowlist: the prototype also refuses `keys[i] = k; i++` and reads of carried state anywhere in
   the body, which finds 31 real leaks tiger misses across the module cache and adds 47 false
   findings to work down first.
7. **Specification reconciliation**, one ticket. Covers the TS-T02 line and Part V table, TS-V01,
   TS-X01's removal, TS-L09's `//nolint` opt-in, the removal of `//tiger:restrict`, and the ENG-181
   gaps (`tiger.yaml`, `tiger golangci --print`, the rule count).
8. **Rewrite the Tiger Explainer** (TODO, in ENG-191 itself, as its last step after the calls are
   decided). The explainer
   says it "describes tiger as it is meant to be when it is finished", and several of the things it
   describes are now decided against. Its overview sells effects and pins as the way a reviewer
   learns that an agent changed the architecture; that goal stays, but pins and computed effects
   are being replaced by the `tiger changed` report (call 16, pending). Rewrite the effects and pins
   sections around the report once call 16 is decided. Also: drop every `//tiger:restrict`
   mention (call 11), replace "tiger flags any `//nolint`" with the opt-in from call 10, say tiger
   keeps no baseline and does not recommend `--new-from-rev` (call 9), describe the map-order rule
   as an allowlist with collect-then-sort and full coverage, a finding as a shape outside it, and
   its five known misses as backed by TS-T11 (calls 8 and 12), remove
   "covers nearly every real loop", and restore the fuller built-versus-described disclaimer that
   `b452818` shortened.
9. **Call 2's ADR**, the first of the per-call ADRs above. Tiger blocks a loop it cannot prove ends, because
   a bound in a loop's header is a claim and the experiments show such claims passing on loops that
   run forever. The way out is a counter cap, so the counter check must be sound and a constant cap
   that stands in for a proof must report when it is hit. False findings on correct code are fixed
   by teaching tiger more proofs, never by stopping the block. This document is the context and the
   ADR is the binding record.
10. **Stop using `//tiger:batched` for loop bounds** (call 13). Write the ADR that replaces
    ADR-0004's loop-bound half and closes ENG-183. Show the restated-limit form in the rule
    reference.
11. **Teach TS-S02 to tell a safety cap from a page size** (call 14), as part of follow-up 1's
    release, and update the spec line and message to state the rule.
12. **The patterns collection** (call 15): rename `examples/` to `patterns/`, clean up and add the
    seven seed patterns, embed them in the binary behind `tiger pattern`, add the registry field and
    the driver's pointer suffix with its meta-tests, run `-race` on `patterns/` in CI, and ship the
    `tiger` binary alongside the plugin.
13. **`tiger changed <base>`** (call 16). Build it from `experiments/changed`: the mixed filter,
    skip generated files, show `context.Context`. Remove `//tiger:effects` and `//tiger:frame`
    with TS-F01, F02, F07, the effect table and the effects and frames analyzers, and the
    effects and frames half of `tiger pin`; ADR-0007 and ADR-0008 are amended to cover variants
    only. ENG-203 settles the output format, including whether new methods on known
    types get their own section and how a brand-new package is summarized.
