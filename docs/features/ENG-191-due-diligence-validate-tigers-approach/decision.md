# ENG-191 Decision: what tiger's static analysis can hold, and the rule set that follows

Date: 2026-09-24, updated 2026-10-01

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

The 2026-10-01 update audited querator's 30 cursor-shaped loops and ran two experiments on how
tiger checks loop bounds. That changed call 2 and added calls 13 and 14, all in section 5.

## The calls in brief

| # | Proposal | Recommendation | If you ratify | If you veto |
|---|---|---|---|---|
| 1 | Add a language model to judge waiver reasons | Reject | tiger stays deterministic; nothing changes | a model judges reasons inside `tiger check` |
| 2 | Keep requiring proof that a loop ends, and make both loop rules sound (TS-V01, TS-S02) | Change | the proof rule keeps blocking and proves more; the 20 holes in `experiments/gaps` close; a constant cap reports when hit | 62 findings on correct code stay, and loops that run forever keep passing |
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
| 13 | Stop using `//tiger:batched` to waive the loop bound (TS-S02, ADR-0004) | Change | store loops restate the limit they already declare; whole-partition scans get a declared maximum or cancellation | the waiver keeps standing in for a limit nobody declared |
| 14 | Tell a safety cap from a page size by the loop's other exit (TS-S02) | Change | a cap that guards an internal condition must report when hit; a page over an outside stream need not | the spec keeps asking for an assert the tool never checks, and silent caps keep passing |

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
answer on their own. They are reproduced inline where they matter. Section 5's two experiments
are committed under [`experiments/`](experiments/), each with its tests and the captured output
of `go test` and `tiger check`.

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

**Recommendation: Reject.** If you ratify, tiger stays deterministic and nothing changes. If you
veto, a model judges waiver reasons inside `tiger check`.

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

### Call 2: keep requiring proof that a loop ends, and make both loop rules sound (TS-V01, TS-S02)

**Recommendation: Change.** If you ratify, a loop still fails the build when tiger cannot prove it
ends. Tiger learns enough new proofs to clear most of the 62 findings on correct code. Both loop
rules stop accepting the loops in `experiments/gaps` that run forever, and a cap that stands in for
a proof has to report when it is hit. If you veto, both loop rules stay as they are, including 62
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

**What ratifying changes.** TS-V01 keeps blocking, and a written `//tiger:variant` is still checked
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

6. **A loop limit has an upper bound where it enters the program.** A parameter or field that is a
   loop's limit carries that fact across packages. Every value passed into it must be a constant,
   another bounded limit, or a value clamped against a declared maximum where it enters, such as
   `min(config.Spins, spinsMax)`. A value read from config, a flag, or a request is clamped at the
   point it is read. A call that passes `math.MaxInt`, the type's maximum, or a constant at least
   half of it is flagged at the call site, since no real limit is that large. For call 13 this
   means querator's `opts.Limit` is clamped to a maximum page size where the request is parsed.
   The tests for changes 5 and 6 are in [`cap_reports.go`](experiments/gaps/cap_reports.go),
   [`config.go`](experiments/gaps/config.go) and
   [their tests](experiments/gaps/cap_reports_test.go).
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

**What remains open.** Two misses are recorded, each with a test that shows the loop running
forever. A constant below the call-site threshold, such as `1 << 40`, passes as a limit even though
no loop reaches it in practice. And a standard-library stream over an endless source inside the
program, like a `bufio.Scanner` over a reader that never ends, counts as an outside stream, so its
limit needs no report. Both leave a visible trail for review. Ranging over a worklist while
appending to it (`WalkRange`) stays a silent bug no loop rule catches; it ends, so it is a
correctness bug for tests.

**What a veto costs.** The 62 findings on correct code stay, and each clears only with a counter cap
like `for budget := len(data); budget > 0 && len(data) > 0; budget-- {`. The 20 loops in
`experiments/gaps` keep passing tiger, and most of them never end. The worklist TS-S01 tells
developers to write keeps running forever on a cycle. The explainer's claim that synthesis "covers
nearly every real loop" stays false on both codebases it has been measured against.

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

**Recommendation: Change.** If you ratify, store loops restate the limit they already declare,
whole-partition scans get a declared maximum or cancellation, and `//tiger:batched` keeps its other
job. If you veto, the waiver keeps standing in for a limit nobody declared.

**What the rule enforces.** TS-S02 requires every loop to state a bound tiger can see. ADR-0004 lets
`//tiger:batched <reason>` waive that on a cursor-shaped loop, reasoning that "the store is finite"
and that any cap would be "either a magic number or a restatement of however big the store is".
The same directive also waives TS-M10, the rule against IO inside a loop.

**Where it applies today.** The audit above shows ADR-0004's premise does not hold for most of
these loops. Seventeen declare a real limit, the page or request size, so restating it is no magic
number. B1 also rejects "the store is finite" as a bound. A finite store with no declared limit is
exactly the limit "chosen by whatever runs out first". ADR-0004 also clashes with TS-V01, which the
spec already records as open item ENG-183.

**What ratifying changes.** `//tiger:batched` stops waiving TS-S02 and stays as the waiver for
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

**What a veto costs.** A reason string keeps standing in for a declared limit. Loops that already
have a limit carry a waiver they do not need. The five scans that cannot be cancelled stay hidden
behind "the table size is the bound", and the clash with TS-V01 stays open.

**Open question, not decided here.** Do filtered loops need a bound on how many times they run, or
is a bound on how many items they return enough? With the restated limit, `count` counts every row
examined, so a namespace filter can return fewer than `opts.Limit` matches even when more exist.
Counting only matches keeps the results right but leaves the run count bounded only by the table's
size. That is a product decision about list semantics, so it is not a call here.

### Call 14: tell a safety cap from a page size by the loop's other exit (TS-S02)

**Recommendation: Change.** If you ratify, tiger decides which caps must report by reading the loop,
and the spec and message say so. If you veto, the spec keeps asking for an assert on every cap while
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

**What ratifying changes.** Tiger reads the loop's other way out (call 2, change 5). When every other
exit drains a stream from outside the module, like `rows.Next()`, the counter is a page size and
needs no report. When the other exit is an internal condition or a `break`, the counter is a safety
cap and must assert or return an error when reached. The spec line and the message state that rule,
and both drop "review judges which kind a cap is". Every loop limit also needs an upper bound where
it enters the program (change 6), so a page size read from a request is clamped to a declared
maximum.

**What a veto costs.** The spec and the tool keep disagreeing, and a cap that stops silently on a
cycle or a stuck condition keeps passing, as `WalkCapped` and `SpinLimited` show in
`experiments/gaps`.

---

## What happens next

### One ADR per decided call

Once the calls here are decided, each one gets its own ADR (architecture decision record) in
`docs/adr/` before any implementation work starts. The ADR records what was decided and why, and
cites this document and its experiments as the context. A vetoed call gets an ADR too, so the
reason for leaving a rule alone is on record. Where a decision replaces part of an existing ADR,
as call 13 does to ADR-0004, the new ADR says so and the old one is marked superseded in part.

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

1. **Make both loop rules sound, then teach TS-V01 more proofs** (call 2), in one release. Fix
   TS-V01's name matching, outside-the-body growth and wraparound; tighten TS-S02's counter check;
   require a safety cap to report when hit, told apart from a page size by the loop's other exit; require every loop limit to be bounded where it enters the program; accept a verified variant, a `ctx.Err()` loop on a
   context parameter, and standard-library iterators; add the four shrink forms. Every loop in
   `experiments/gaps`, `experiments/variants` and `experiments/annotations` becomes an analyzer test
   case. Fix tiger's own 14 worklist loops and TS-S01's rule reference and fixture. Rerun both
   codebases and record what is left.
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
