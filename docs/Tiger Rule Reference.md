# Tiger Rule Reference

Every rule tiger enforces, one entry each. A finding line names its rule code (`TS-S02`,
`TS-C05`). Look the code up here to learn what the rule wants, why it exists, and the edit
that satisfies it. The finding line itself stays terse on purpose. The reasoning lives here,
and the normative depth lives in [the specification](Tiger%20Specification.md).

If you have never run tiger, start with [the explainer](Tiger%20Explainer.html) instead.
It teaches the concepts this reference assumes.

## How to read a finding

Every finding is one line with two halves.

```
internal/handlers.go:117:2: TS-S02: this for loop has no condition and no select on ctx.Done(), so nothing bounds it — add a cap (for tries := 0; tries < max; tries++) that fails when the cap is hit, or select on ctx.Done() inside it
```

The head (`path:line:col: TS-XXX:`) is a parsing contract. Tools key on it and it never
changes. The body is prose in up to three parts. What the code does, what tiger expected,
and the edit to make. The body never uses tiger jargon and spells directives out literally,
so you can act on it without this reference. This reference adds the why.

## Blocking and advisory

A rule has one of two severities, and nothing in between.

**Blocking** findings fail the run. `tiger check` exits 1 and the line prints. There is no
warning tier. If tiger prints a finding, the build is red until the code changes or an
analyzer bug gets fixed. A false positive on a blocking rule is a bug in tiger. File it,
and the case lands in the rule's test corpus so it cannot regress.

**Advisory** findings are standing accounting, not lesser warnings. Two things stand on
purpose in real code (an escape directive, a skipped test), and tiger counts them instead
of pretending they are errors. The counts are checked against a per-package budget. Under
budget they print nothing. Over budget the run fails. The next section explains the file
that holds those budgets.

## Budgets and the ratchet

`tiger.budget.yaml` at the module root holds one integer per package per advisory rule.
The number is how many findings of that rule the package may carry. A package with
findings and no row has a budget of zero, so the first run after adoption fails until
`tiger budget --write` records the current counts once.

The tool only lowers numbers. `tiger budget --write` shrinks a row to the current count,
creates a missing row, or deletes a row that reaches zero. It has no operation that raises
one. Raising a budget is a hand edit that shows up in a pull request diff for a reviewer
to rule on. That is the ratchet (rule TS-D06). Debt can only shrink without a human
signing off on growth.

On an overrun, the package's counted findings print as blocking lines under a `TS-D06`
line, and the run fails. Under budget, nothing prints. Check output is a verdict either
way.

Budgets are enforced by the `tiger` CLI only. The golangci-lint plugin has no end-of-run
step to aggregate counts, so it reports every counted finding as an issue. A repo that
carries budgeted debt needs the CLI.

## Directives

A directive is a `//tiger:<verb>` comment. It binds to the code that starts on the line
after its comment group. The vocabulary is closed. An unknown verb is a blocking finding
(TS-L09), so a typo cannot silently disable anything. There are three kinds.

**Escape hatches** loosen a rule at one site, or state a claim tiger cannot check, and
always carry a reason. `batched` exists because some external systems only accept one item
at a time (a fact of the world no rewrite removes). `openenum` claims a type's set of values
is open to growth, which no analyzer can verify. Every escape is counted against its
package's budget on every run, whether it waives anything or not.

**Pins** freeze a fact tiger computed (what a function does, what it writes, why a loop
ends) into a contract. Absence of a pin is never a finding. `tiger pin <Name>` writes one;
after that, any drift from the pinned fact blocks.

**Intent declarations** state something no analyzer can compute, and analyzers hold the
code to the statement.

| Verb | Kind | Rules it affects |
|---|---|---|
| `//tiger:batched <reason>` | escape | TS-M10 (per-item IO in a loop); TS-S02 (cursor-shaped loops only); counted by TS-L09 |
| `//tiger:openenum <reason>` | escape | TS-S08 (default arm instead of `assert.Unreachable`); counted by TS-L09 |
| `//tiger:effects <set>` | pin | TS-F01, TS-F02 |
| `//tiger:frame <set>` | pin | TS-F07 |
| `//tiger:variant <expr>` | pin | TS-V01 |
| `//tiger:requires <cond>` | pin | TS-V03 |
| `//tiger:ensures <cond>` | pin | TS-V03 |

## Bounds and control flow

<!-- rule: TS-S01 -->
### TS-S01 — no recursion; cycles in the static call graph are findings

**Severity:** blocking — a finding fails the run.

A recursive function's depth depends on its input data, not on anything in the code. Go
lets a single call stack grow up to one gigabyte before it gives up. So a bad input does
not fail fast. It quietly eats memory and crashes far from the real cause.

Code that fires it:

```go
type Node struct {
	Children []*Node
}

func chase(n *Node, visit func(*Node)) {
	if n == nil {
		return
	}
	visit(n)
	for _, child := range n.Children {
		chase(child, visit)
	}
}
```

```
tree.go:5:6: TS-S01: chase → chase is a cycle of calls (recursion) — replace it with a capped worklist loop (for i := 0; i < len(work) && i < max; i++ { work = append(work, next...) }) that returns an error when it stops at max with nodes left
```

The compliant rewrite:

```go
const nodesMax = 1 << 20

var errTooManyNodes = errors.New("walk reached nodesMax")

func chaseIterative(root *Node, visit func(*Node)) error {
	if root == nil {
		return nil
	}
	stack := []*Node{root}
	i := 0
	for ; i < len(stack) && i < nodesMax; i++ {
		visit(stack[i])
		stack = append(stack, stack[i].Children...)
	}
	if i == nodesMax && i < len(stack) {
		return errTooManyNodes
	}
	return nil
}
```

The worklist grows the slice it walks, so on a graph with a cycle it never ends.
TS-S02 and TS-V01 both block the uncapped form `for i := 0; i < len(stack); i++`, and
accept the capped one. The report is `i == nodesMax && i < len(stack)`: a graph with
exactly `nodesMax` nodes ends here too, with nothing left to visit, and must pass.

<!-- rule: TS-S02 -->
### TS-S02 — every loop has an upper bound

**Severity:** blocking — a finding fails the run.

Every loop should state how many times it can run. A loop with no stated limit assumes
its input is always well-formed. State a bound, and a bad input becomes a fast, loud
failure instead of a silent hang. This rule caught five real bugs in trials on two
codebases, including a remotely triggerable denial of service in a git server (two tags
pointing at each other spun a worker goroutine at full speed forever).

A loop that fires it:

```go
func newton(estimate float64) float64 {
	for !converged(estimate) {
		estimate = refine(estimate)
	}
	return estimate
}
```

```
newton.go:2:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
newton.go:2:6: TS-S02: tiger can't tell how many times this loop runs: its condition is not a constant, a len, or a counter — add a cap (for tries := 0; tries < max; tries++) that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
```

The compliant rewrite:

```go
const newtonItersMax = 20

func newton(estimate float64) float64 {
	for i := 0; i < newtonItersMax; i++ {
		if converged(estimate) {
			break
		}
		estimate = refine(estimate)
	}
	assert.Ok(converged(estimate), "no convergence within newtonItersMax")
	return estimate
}
```

A bound is a constant, a `len`, a sound counter, or a variant that TS-V01 verified
(whether tiger found it or a developer pinned it).

A counter is sound only when all of this holds:

- its comparison is a top-level `&&` term of the condition, not inside `||` or behind `!`
- it pairs `<` or `<=` with `++` or `+= <constant>`, or `>` or `>=` with `--` or
  `-= <constant>`, never `!=`
- nothing in the body writes the counter or its limit, whether through a helper, a method,
  a pointer, a closure, or a map insert when the limit is `len(m)`
- the counter arrives at the limit before it wraps around its type's range, so
  `for i := uint8(0); i <= 255; i++` is not sound

A limit may call only `len`, `cap`, `min`, `max`, conversions, and declared functions or
methods such as `tuple.Len()`. A function value could return something larger on every
pass.

A counter cap that stands in for a proof must report when it is hit. A counter is a
safety cap when its limit is not a `len` or `cap` and the loop has another way out: a
second `&&` term or a `break`. A `return` from the body is a result, so it does not count. The loop must then assert, or return an error, when the
counter reaches the limit:

```go
func spinCapped(done func() bool, limit int) {
	for i := 0; i < limit && !done(); i++ {
	}
}
```

```
spin.go:2:14: TS-S02: this counter cap ends the loop without saying so, so a truncated result passes as complete — declare the counter before the loop and return an error or assert when it reaches the limit
```

```go
func spinReported(done func() bool, limit int) error {
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errSpinCapHit
	}
	return nil
}
```

The report is an `if` after the loop that compares the counter with the limit, or reads a
variable the loop assigns, and then asserts, panics, or returns an error. A bare assert or
panic statement after the loop counts too.

A counter cap is a page size, and needs no report, when its limit is not a constant and
every other way out of the loop drains a stream whose type is declared outside the module
(`rows.Next()`, `scanner.Scan()`). Reaching a page size means the page is full.

`for ctx.Err() == nil` on a `context.Context` parameter is a forever loop, like a
`select` on `ctx.Done()`. On a context built inside the function, such as
`context.Background()`, it is a finding. Ranging over an iterator is bounded only when it
comes from the standard library.

**Directives:** `//tiger:batched <reason>` waives this rule only on a cursor-shaped
loop. That is one whose condition is a boolean method call that advances itself, like
`for it.Valid()` or `for rows.Next()`. The store behind it is finite, which is why the
waiver applies. Every use still counts against the package's escape-directive budget.

**Known misses:** A limit passed through a function value or an interface method is not
traced. A constant below half the type's maximum, such as `1 << 40`, passes as a limit. A
standard library stream over an endless in-program source counts as an outside stream.
Ranging over a slice while appending to it ends, so it stays a correctness bug for
tests, not a loop rule. A limit read through a declared method (`q.Len()`) is trusted, so a
body that grows `q` through another method, or a declared function that grows a global it
returns, goes unseen. A struct field changed through a global pointer or an interface method
that the loop never passes the struct to goes unseen too. A cap's report is any failing `if`
on a variable the loop assigns, so an unrelated check such as `if err != nil` counts.

<!-- rule: TS-S02-limit -->
### TS-S02 (limit) — every loop limit has an upper bound where it enters the program

**Severity:** blocking — a finding fails the run.

A loop that counts to a number is only as bounded as the number. A page size read from
a request, a spin count from a config file, or an object count from a binary header lets
the outside world pick how long the loop runs. Tiger traces each loop limit back to
where it enters the program, through parameters, struct fields, local variables and
function results, across packages. Every value written into it must be a constant, a
length, another bounded limit, or a value clamped against a declared maximum, such as
`min(config.Spins, spinsMax)` or an `if count > objectsMax` that rejects. A function that
returns a clamped value carries the bound to its callers. A clamp caps the upper end only, so
subtracting anything but a non-negative constant, length or unsigned value from a limit, negating it,
multiplying or taking an exclusive or of two values that may both be negative, or dividing it by a value that may be negative makes it
unbounded again. Wrap-around at the bottom of the type is not modelled. A parameter the function writes carries the written value as well as the caller's. A constant at least half of the
type's maximum, such as `math.MaxInt`, is a finding too, because no loop reaches it. That
applies to integer types of 32 bits or wider. Standard library packages and `_test.go` files
are not traced, and a running total (`n += len(x)`, `n++`) counts as bounded.

A loop that fires it:

```go
func spin(config Config, done func() bool) error {
	return spinReported(done, config.Spins)
}
```

```
spin.go:2:28: TS-S02: this value reaches a loop limit with no upper bound where it enters — clamp it there, for example min(x, xMax), or reject it with if x > xMax
```

The compliant rewrite:

```go
const spinsMax = 10_000

func spin(config Config, done func() bool) error {
	return spinReported(done, min(config.Spins, spinsMax))
}
```

A constant no loop reaches gets its own message:

```go
func spin(done func() bool) error {
	return spinReported(done, math.MaxInt)
}
```

```
spin.go:2:28: TS-S02: this value is too large for the loop to ever reach, so it bounds nothing — pass a declared maximum instead
```

Facts from other packages are read only when those packages are part of the run: a
clamp in another package counts under `tiger check ./...` and is missed under
`tiger check ./one/package`.

**Known misses:** A limit passed through a function value or an interface method is not
followed. A constant below half the type's maximum, such as `1 << 40`, passes. A clamp
that sits in another function and rejects there, rather than in the function that
passes the value, is not seen, so the call is flagged.

<!-- rule: TS-S03 -->
### TS-S03 — an unbounded event loop selects on ctx.Done()

**Severity:** blocking — a finding fails the run.

Some loops must run forever, like a server's main event loop. Give that loop a way to
stop. Then shutdown works, and a stuck loop is a bug you can reach instead of a core you
have to kill.

A loop that fires it:

```go
func runNoTermination(input <-chan int, output chan<- int) {
	for {
		select {
		case value := <-input:
			output <- value * 2
		}
	}
}
```

```
worker.go:3:3: TS-S03: this event loop's select has no case that stops the loop — add case <-ctx.Done(): return, or a case on a shutdown channel (struct{}-typed or named like one), so the loop can end
```

The compliant rewrite:

```go
func runWithTermination(ctx context.Context, input <-chan int, output chan<- int) {
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value * 2
		}
	}
}
```

**Known miss:** A `struct{}`-typed channel used only as a concurrency semaphore can be
mistaken for a shutdown signal, leaving a genuinely unbounded loop unflagged.

<!-- rule: TS-S06 -->
### TS-S06 — one logical operator per condition

**Severity:** blocking — a finding fails the run.

A condition that mixes `&&` and `||` forces the reader to work out precedence in their
head. Splitting it into nested if/else makes each case explicit. It also gives you a
place for the else branch you might otherwise forget.

Code that fires it:

```go
func mixedIf(a, b, c bool) bool {
	if a && b || c {
		return true
	}
	return false
}
```

```
check.go:2:5: TS-S06: this condition mixes && and ||, so the reader has to work out precedence — split it into nested if/else with one operator per condition
```

The compliant rewrite:

```go
func mixedIf(a, b, c bool) bool {
	if a && b {
		return true
	}
	if c {
		return true
	}
	return false
}
```

<!-- rule: TS-S07 -->
### TS-S07 — split compound assertions

**Severity:** blocking — a finding fails the run.

One assertion that checks two things with `&&` only tells you the pair failed. It does
not say which half broke. Splitting it into two assertions names the exact check that
failed.

Code that fires it:

```go
func checkPairOk(a, b bool) {
	assert.Ok(a && b, "a and b")
}
```

```
check.go:2:12: TS-S07: this assert.Ok checks two things joined by && — split it into one assert.Ok per check so a failure names which one broke
```

The compliant rewrite:

```go
func checkPairOk(a, b bool) {
	assert.Ok(a, "a")
	assert.Ok(b, "b")
}
```

<!-- rule: TS-S08 -->
### TS-S08 — a switch over a closed set ends in assert.Unreachable

**Severity:** blocking — a finding fails the run.

A type can have a fixed, known set of values. Adding a new value should break every
switch that ignores it, but Go's compiler cannot enforce that, because such constants
are just integers. Ending each switch in a default arm that calls `assert.Unreachable`
turns a missed case into a loud crash. This rule caught three real bugs in trials. Two
were storage backends silently dropping an action with no default case, and one was a
pack writer emitting an invalid type code for an unexpected value.

Code that fires it:

```go
type Status int

const (
	StatusActive Status = iota
	StatusInactive
)

func classify(status Status) string {
	switch status {
	case StatusActive:
		return "active"
	case StatusInactive:
		return "inactive"
	}
	return ""
}
```

```
status.go:9:2: TS-S08: this switch over Status has no default arm — add default: assert.Unreachable("Status: unhandled value") so a new Status constant fails loudly
```

The compliant rewrite:

```go
func classify(status Status) string {
	switch status {
	case StatusActive:
		return "active"
	case StatusInactive:
		return "inactive"
	default:
		assert.Unreachable("unhandled Status")
	}
	return ""
}
```

**Directives:** `//tiger:openenum <reason>` marks a type's set of values as deliberately open to
growth. A switch over that type still needs a default arm, just not one ending in
`assert.Unreachable`. The directive needs a reason and counts against its package's budget
(TS-L09); one without a reason does not parse, so it waives nothing. Tiger recognizes the directive only when the switch is in the same
package as the marked type.

**Known miss:** A switch over an `//tiger:openenum` type in a different package from the
type does not get the waiver.

<!-- rule: TS-S09 -->
### TS-S09 — no goto, no labeled break or continue

**Severity:** blocking — a finding fails the run.

Goto and labeled jumps hide control flow. A reader must trace the label by hand to see
where execution goes next. A labeled break out of a loop usually means the inner loop
wants to be its own function. Extracting it keeps the code reading top to bottom.

Code that fires it:

```go
func firstMatch(rows [][]int, needle int) bool {
	found := false
scan:
	for _, row := range rows {
		for _, cell := range row {
			if cell == needle {
				found = true
				break scan
			}
		}
	}
	return found
}
```

```
search.go:8:5: TS-S09: this labeled break jumps out of an inner loop to an outer one — move the inner loop into its own function and return instead
```

The compliant rewrite:

```go
func firstMatch(rows [][]int, needle int) bool {
	for _, row := range rows {
		if rowContains(row, needle) {
			return true
		}
	}
	return false
}

func rowContains(row []int, needle int) bool {
	for _, cell := range row {
		if cell == needle {
			return true
		}
	}
	return false
}
```

<!-- rule: TS-S18 -->
### TS-S18 — no naked panic outside the assert package

**Severity:** blocking — a finding fails the run.

A bare panic invents its own crash path with its own message format. Tiger wants one way
a program crashes, so every crash reads the same and carries the same context. Route
hard failures through `assert.Ok`, `assert.Fail`, or `assert.Unreachable` instead.

Code that fires it:

```go
func applyEntry(size int) {
	if size < 0 {
		panic("negative size")
	}
}
```

```
entry.go:3:3: TS-S18: panic is called directly here — use assert.Ok (condition), assert.Fail (formatted failure), or assert.Unreachable (impossible arm) instead so every crash goes through one path
```

The compliant rewrite:

```go
func applyEntry(size int) {
	assert.Ok(size >= 0, "entry size is negative")
}
```

<!-- rule: TS-S21 -->
### TS-S21 — every limit constant participates in a relational assertion

**Severity:** blocking — a finding fails the run.

Nobody can check whether a limit number is correct just by reading the code. Whether
8189 is the right number depends on your hardware, not your logic. What tiger can check
is whether you related that limit to another quantity. A limit related to nothing is a
number nobody actually reasoned about.

Code that fires it:

```go
const batchMax = 8189
```

```
limits.go:1:7: TS-S21: batchMax is a limit that no assertion relates to any other quantity, so nothing checks it makes sense — add a compile-time relation such as const _ = uint(bufferSize - batchMax), which stops compiling once batchMax outgrows what bounds it
```

The compliant rewrite:

```go
const writeBufferSize = 32768
const headerSizeBytes = 12
const entrySize = 4

const batchMax = 8189

// Stops compiling if batchMax outgrows the buffer that must hold a batch.
const _ = uint((writeBufferSize - headerSizeBytes) - batchMax*entrySize)
```

<!-- rule: TS-S22 -->
### TS-S22 — a limit constant's stated derivation evaluates to its value

**Severity:** blocking — a finding fails the run.

A limit's derivation is the back-of-envelope math that produced it, written as a real
expression instead of prose. Tiger evaluates that expression and checks it still equals
the constant. If someone changes an input number, this catches a limit that drifted out
of date.

Code that fires it:

```go
const writeBufferSize = 32768
const headerSizeBytes = 12
const entrySize = 4

// batchMax = (writeBufferSize - headerSizeBytes) / entrySize.
const batchMax = 8188
```

```
limits.go:6:7: TS-S22: batchMax is 8188 but the derivation in its comment, (writeBufferSize - headerSizeBytes) / entrySize, works out to 8189 — the two drifted apart; recompute the constant or correct the comment
```

The compliant rewrite:

```go
const writeBufferSize = 32768
const headerSizeBytes = 12
const entrySize = 4

// batchMax = (writeBufferSize - headerSizeBytes) / entrySize.
const batchMax = 8189
```

## Concurrency

<!-- rule: TS-C02 -->
### TS-C02 — every goroutine starts through a supervisor

**Severity:** blocking — a finding fails the run.

A goroutine nobody owns is a leak that keeps running after its caller moves on. Nobody
notices until a server's memory climbs for no clear reason. Starting goroutines through
a supervisor, such as `errgroup.Group.Go`, ties each one to code that tracks when it
should stop. This rule caught a real bug in the trials. A daemon's WaitGroup went
unwaited, so Shutdown could return before its HTTP goroutines exited.

Code that fires it:

```go
func Launch() {
	go run()
}
```

```
worker.go:4:2: TS-C02: this go statement starts a goroutine nobody owns: nothing says when it exits or ties it to a context — start it through errgroup.Group.Go
```

The compliant rewrite:

```go
func Launch(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return run(ctx)
	})
	return g.Wait()
}
```

<!-- rule: TS-C05 -->
### TS-C05 — every blocking receive or send has a ctx.Done() case

**Severity:** blocking — a finding fails the run.

A blocking wait with no way out forces shutdown to hang until something kills the
process, losing data mid-write. This rule caught a real bug in the trials. A queue's
Produce, Complete, and Retry waits could never be cancelled, while the doc comment
claimed cancellation that only the Lease method actually implemented. The check accepts
a shutdown channel in place of `ctx.Done()`. That means a channel typed `struct{}`, or
one named shutdown, stop, quit, or done. Do not wrap those in a redundant extra case.

Code that fires it:

```go
func awaitBlocking(values chan int) {
	select {
	case <-values:
	}
}
```

```
queue.go:4:2: TS-C05: this select blocks with no case that ends the wait on shutdown — add case <-ctx.Done(): return ctx.Err()
```

The compliant rewrite:

```go
func awaitWithShutdown(ctx context.Context, values chan int) (int, error) {
	select {
	case v := <-values:
		return v, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
```

<!-- rule: TS-C09 -->
### TS-C09 — no spawning work in direct reaction to an external event

**Severity:** blocking — a finding fails the run.

Spawning one goroutine per incoming item lets outside events set the code's pace, not
the code itself. A burst of requests becomes a burst of goroutines, with no limit on how
many run at once. Putting items in a bounded queue and draining it from one worker gives
you batching and back pressure. The queue's size becomes the one place that decides how
fast work happens.

Code that fires it:

```go
func Notify(entries []string) {
	for _, entry := range entries {
		go notifyOne(entry)
	}
}
```

```
dispatcher.go:5:3: TS-C09: this loop starts a goroutine per item, so the goroutines react to each item instead of working at their own pace — append the items to a bounded queue and drain it from one supervisor goroutine
```

The compliant rewrite:

```go
const queueCapacity = 64

func Notify(entries []string) {
	queue := make([]string, 0, queueCapacity)
	queue = append(queue, entries...)
	drain(queue)
}

func drain(queue []string) {
	for _, entry := range queue {
		notifyOne(entry)
	}
}
```

**Not checked in tests:** `_test.go` files are skipped for this rule. A concurrency test
starts one goroutine per request on purpose, and a single-worker queue would defeat it.
The same loop in a non-test file still fires. A goroutine started outside a loop in a test
is still TS-C02.

<!-- rule: TS-C12 -->
### TS-C12 — channel types are declared in one file per package

**Severity:** blocking — a finding fails the run.

When every channel type used across goroutines lives in one file, a reviewer sees the
whole communication map. Capacity constants end up next to each other too, so a
mismatched pair stands out immediately. One channel sized for ten items next to another
sized for a hundred is a mismatch worth catching. Scattering the types across files
hides it until it causes a slowdown or a deadlock.

Code that fires it (`channels.go` already exists in the package and holds the package's
channel types):

```go
// worker.go
package worker

// Jobs carries units of work between the dispatcher and worker goroutines.
type Jobs chan Job
```

```
worker.go:4:6: TS-C12: this channel type is declared outside channels.go, the file that holds this package's channel types — move the declaration to channels.go
```

The compliant rewrite (moved into `channels.go`):

```go
// channels.go
package worker

// Jobs carries units of work between the dispatcher and worker goroutines.
type Jobs chan Job
```

**Known miss:** A channel type spelled out only in a function signature or a `var`
declaration, never given its own named type, is not counted, so it can sit anywhere.


## Errors

<!-- rule: TS-E02 -->
### TS-E02 — discarding a result with _ requires a justification comment

**Severity:** blocking — a finding fails the run.

Throwing away an error with an underscore is sometimes the right call. Not every failure
needs handling, and banning the discard outright just pushes people toward a silencing
`//nolint` comment instead. Requiring a comment keeps the reasoning visible, so a
reviewer can judge whether discarding was actually safe. This rule caught a latent bug
in the trials. A hash function discarded a validation error, set to return corrupted
zero-value IDs the day a second hash format shipped.

Code that fires it:

```go
func closeQuiet(f *os.File) {
	_ = f.Close()
}
```

```
cleanup.go:6:2: TS-E02: this error is discarded with _, so a failure here goes unnoticed — handle the error, or say in a comment on this line or the line above why ignoring it is safe
```

The compliant rewrite:

```go
func closeQuiet(f *os.File) {
	_ = f.Close() // best-effort cleanup; callers already check write errors
}
```

**Cleanup is exempt:** a discarded error inside a `defer` statement's call, or inside a
function literal passed to `t.Cleanup`, is not a finding. Closing something on the way out
has no better answer than dropping the error, so a comment there would add nothing.
Cleanup that is not deferred, like `_ = resp.Body.Close()`, still fires.

**Known miss:** The check verifies a comment exists next to the discard, not that it is
true. `_ = f.Close() // ok` satisfies it; the truth of the reason is review's job.

<!-- rule: TS-E06 -->
### TS-E06 — minimize return arity

**Severity:** blocking — a finding fails the run.

Every value a function returns is one more thing the caller has to check and branch on.
Those branches spread to every caller up the chain. Keeping return values to at most
two, with the second an error or a bool, keeps that spread manageable. A three-value
return like `(T, bool, error)` forces the caller to juggle two different kinds of
failure at once.

Code that fires it:

```go
func Divide(a, b int) (int, bool, error) {
	if b == 0 {
		return 0, false, errors.New("divide by zero")
	}
	return a / b, true, nil
}
```

```
math.go:5:23: TS-E06: this function returns 3 results — return at most two: nothing, T, (T, bool), or (T, error); never (T, bool, error)
```

The compliant rewrite:

```go
func Divide(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("divide by zero")
	}
	return a / b, nil
}
```


## Layout and directives

<!-- rule: TS-L05 -->
### TS-L05 — struct order is fields, nested types, constructor, methods

**Severity:** advisory — findings are counted against the package's row in tiger.budget.yaml: silent under budget; on overrun they print as blocking lines under a TS-D06 line. See [Budgets and the ratchet](#budgets-and-the-ratchet).

A reader moves through a file top to bottom. Fields should come before the constructor,
and the constructor before methods. A method sitting above its type makes a reader meet
behavior before they know what data it acts on. Keeping the order fixed makes every
struct's layout predictable.

Code that fires it:

```go
func (s *Server) Ping() string {
	return "pong " + s.Addr
}

type Server struct {
	Addr string
}
```

```
server.go:1:1: TS-L05: method Ping is declared before its type Server — move Server's type declaration above its methods so a reader meets the fields first
```

The compliant rewrite:

```go
type Server struct {
	Addr string
}

func (s *Server) Ping() string {
	return "pong " + s.Addr
}
```

**Known miss:** The analyzer compares declarations within one file. A method for a type
declared in a different file stays silent, even when the split is accidental.

<!-- rule: TS-L09 -->
### TS-L09 — every directive parses and every escape carries a reason

**Severity:** blocking — a finding fails the run.

A `//tiger:` comment is a promise to the checker (a pin, an intent declaration, or an
escape). If one does not parse (a made-up verb, a missing reason, garbled arguments),
whatever it was meant to do would silently not happen. So a malformed directive is
itself a blocking finding, and an escape without a reason never enters the code.

Code that fires it:

```go
func notifyAll(hooks []string) {
	//tiger:batched
	for range hooks {
		notifyOne()
	}
}
```

```
directive.go:2:2: TS-L09: //tiger:batched has no reason after it — write the outside-world constraint that forces this shape, for example //tiger:batched provider offers no bulk endpoint
```

The compliant rewrite:

```go
func notifyAll(hooks []string) {
	//tiger:batched provider offers no bulk endpoint; contract caps us at 10 rps
	for range hooks {
		notifyOne()
	}
}
```

**Directives:** This rule validates every directive in the file. A well-formed escape is
not blocking on its own; the entry below counts it instead.

**Banning `//nolint`:** tiger never honors `//nolint`, and does not ban it by default. A
team that wants every `//nolint` gone turns the ban on in `tiger.yaml`:

```yaml
version: 1
nolint:
  forbid: true
  reason: agents must fix findings, not silence them
```

With the entry on, `tiger check` reports every `//nolint` in a checked package as a blocking
TS-L09 finding at the file's `package` line, where a `//nolint` on the original line cannot
cover it. A `//nolint` in a generated file is not reported:

```
store.go:2:1: TS-L09: line 14 has a //nolint, and this repository's tiger.yaml forbids them (nolint.forbid) — fix the finding it silences instead
```

The ban holds under `tiger check` only. Under the golangci-lint plugin, a file-wide
`//nolint:tiger` above the `package` clause or an `exclusions` rule in `.golangci.yml` drops
the finding, so CI relies on `tiger check`.

<!-- rule: TS-L09-escape -->
### TS-L09 (escape accounting) — every escape directive is counted

**Severity:** advisory — findings are counted against the package's row in tiger.budget.yaml: silent under budget; on overrun they print as blocking lines under a TS-D06 line. See [Budgets and the ratchet](#budgets-and-the-ratchet).

An escape like `//tiger:batched <reason>` waives one rule at one spot, and sometimes
that is the right call. `//tiger:openenum <reason>` is counted the same way, because tiger
cannot check that an enum really is open. But every escape loosens a rule, so tiger counts each one
instead of letting it vanish once approved. The count sits in the package's budget,
where a team can see exactly how many escapes it carries and a raise needs review.

Code that fires it:

```go
func notifyAll(hooks []string) {
	//tiger:batched provider offers no bulk endpoint; contract caps us at 10 rps
	for range hooks {
		notifyOne()
	}
}
```

```
directive.go:2:2: TS-L09: //tiger:batched "provider offers no bulk endpoint; contract caps us at 10 rps" waives a rule here; tiger cannot check the claim, so this notice stands on every run — remove the directive when the constraint no longer holds
```

The compliant rewrite:

```go
func notifyAll(hooks []string) {
	for range hooks {
		notifyOne()
	}
}
```

**Directives:** TS-L09 checks that the directive parses and carries a reason. This half
counts every escape that passes that check, for as long as it stays in the code.

<!-- rule: TS-L10 -->
### TS-L10 — no defer inside a loop

**Severity:** blocking — a finding fails the run.

A `defer` schedules cleanup for when the whole function returns, not the current loop
iteration. Put one inside a loop, and every iteration queues its own cleanup instead of
releasing right away. On a long loop that is a pile of unreleased resources, all freed
at once at the end.

Code that fires it:

```go
func closeAll(names []string) error {
	for i := 0; i < len(names); i++ {
		r, err := openResource()
		if err != nil {
			return err
		}
		defer r.Close()
	}
	return nil
}
```

```
resource.go:7:3: TS-L10: this defer sits inside a loop, so its cleanup waits until the whole function returns, once per iteration — move the loop body into its own function so the defer runs at the end of each call
```

The compliant rewrite:

```go
func closeOne(name string) error {
	r, err := openResource()
	if err != nil {
		return err
	}
	defer r.Close()
	return nil
}

func closeAll(names []string) error {
	for _, name := range names {
		if err := closeOne(name); err != nil {
			return err
		}
	}
	return nil
}
```

<!-- rule: TS-L10-distance -->
### TS-L10 (defer placement) — a defer stays next to its acquisition

**Severity:** advisory — findings are counted against the package's row in tiger.budget.yaml: silent under budget; on overrun they print as blocking lines under a TS-D06 line. See [Budgets and the ratchet](#budgets-and-the-ratchet).

When a `defer` sits right after the call that acquired the resource, a reader sees both
together. Move the defer away, behind an unrelated line or inside an `if`, and that link
gets harder to see. The code still works, but a leak becomes easier to miss.

Code that fires it:

```go
func report(name string) error {
	r, err := openResource()
	if err != nil {
		return err
	}
	println("opened", name)
	defer r.Close()
	return nil
}
```

```
resource.go:7:2: TS-L10: this defer is not right after the call that acquired what it releases — move it to the line after the acquisition (or after its if err != nil check) so acquire and release read together
```

The compliant rewrite:

```go
func report(name string) error {
	r, err := openResource()
	if err != nil {
		return err
	}
	defer r.Close()
	println("opened", name)
	return nil
}
```

**Known miss:** The analyzer recognizes a defer whose call is a selector on a plain
identifier, like `r.Close()` or `tx.Rollback()`. A deferred closure or a deferred call
to a package-level function never resolves to an acquisition, so those shapes stay
silent.


## Memory and IO

<!-- rule: TS-M05 -->
### TS-M05 — pooled types implement Reset, and Put is preceded by a reset

**Severity:** blocking — a finding fails the run.

Putting a value back into a `sync.Pool` without clearing it first leaves leftover data
for the next caller. Go zeroes memory it allocates fresh, but a pooled value is reused,
not freshly allocated, so that protection does not apply. Clearing the value right
before every `Put` closes the gap, and the owner who knew what was sensitive does the
clearing, not some later caller.

Code that fires it:

```go
func record(entry byte) {
	buf, ok := bufferPool.Get().(*Buffer)
	if !ok {
		return
	}
	buf.data = append(buf.data, entry)
	bufferPool.Put(buf)
}
```

```
pool.go:7:2: TS-M05: this Put is not preceded by a Reset (or a zeroing) of the same value on every path to it — call Reset() or zero the value before every Put, so pooled data can't leak to the next user
```

The compliant rewrite:

```go
func record(entry byte) {
	buf, ok := bufferPool.Get().(*Buffer)
	if !ok {
		return
	}
	buf.data = append(buf.data, entry)
	buf.Reset()
	bufferPool.Put(buf)
}
```

**Known miss:** The analyzer looks inside one function. A reset done by the caller
before handing the value to a Put-only helper is invisible, and a Put reached through an
alias of the same value is invisible too.

<!-- rule: TS-M10 -->
### TS-M10 — no IO inside a loop body

**Severity:** blocking — a finding fails the run.

Calling a network, disk, or database once per loop iteration is slow. Each call pays its
own round trip instead of sharing one. This is the classic "N+1 queries" mistake, where
one query becomes one per item. Move the call above the loop or batch it, so the round
trip happens once. When the outside system genuinely offers no bulk operation, that is
what the escape below is for.

Code that fires it:

```go
func readAll(paths []string) error {
	for i := 0; i < len(paths); i++ {
		_, err := os.ReadFile(paths[i])
		if err != nil {
			return err
		}
	}
	return nil
}
```

```
fetch.go:3:13: TS-M10: this call does IO (network, disk, or database) and runs once per loop iteration — move it above the loop, or if each item genuinely needs its own IO, mark the loop //tiger:batched <reason>
```

The declared escape, when each item genuinely needs its own IO:

```go
func readAll(paths []string) error {
	//tiger:batched files arrive as an external drop directory; there is no bulk read
	for i := 0; i < len(paths); i++ {
		_, err := os.ReadFile(paths[i])
		if err != nil {
			return err
		}
	}
	return nil
}
```

**Directives:** `//tiger:batched <reason>` waives per-item IO in a loop when the outside
system has no bulk endpoint to call instead. The directive is still counted against the
package's budget as a TS-L09 escape.

**Known miss:** IO hidden behind a same-package helper is invisible. The analyzer checks
direct calls, not a call graph, so a loop calling a local helper that itself calls
`os.ReadFile` is missed.


## Naming

<!-- rule: TS-N07 -->
### TS-N07 — no adjacent same-type parameters, at most four parameters

**Severity:** blocking — a finding fails the run.

Go has no named arguments, so a call like `Copy(a, b, 3, 10)` gives no protection.
Nothing stops a caller from swapping two same-typed values by mistake. The compiler
cannot catch it, because both fit the type. Grouping same-typed parameters into a named
options struct forces every value to be named at the call site.

Code that fires it:

```go
func connect(retries, timeout int) {
	_, _ = retries, timeout
}
```

```
copy.go:1:29: TS-N07: adjacent parameters share type int, so a caller can swap them and nothing complains — use an options struct, or give each a distinct named type
```

The compliant rewrite:

```go
type connectOptions struct {
	Retries int
	Timeout int
}

func connect(options connectOptions) {
	_ = options
}
```

**Leading `t` and `ctx` are free:** a leading `*testing.T`, `*testing.B` or `testing.TB`
does not count toward the four-parameter limit, and neither does a `context.Context` that
comes first or right after it. A `t` or `ctx` anywhere else still counts. The
adjacent-same-type check counts every parameter.

**Known miss:** Only parameters directly next to each other are flagged. Two same-typed
parameters separated by a different type stay silent. Function literals are exempt too,
because their signatures are usually fixed by the API consuming them (`sort.Slice`'s
comparator is the classic case).

<!-- rule: TS-N08 -->
### TS-N08 — no bool parameters; use a named type

**Severity:** blocking — a finding fails the run.

A plain `true` or `false` at a call site like `Save(true)` tells the reader nothing
about what it controls. They have to jump to the function signature to find out. A named
type with named constants reads clearly right where it is called, with no lookup needed.

Code that fires it:

```go
func save(immediate bool) {
	_ = immediate
}
```

```
save.go:1:11: TS-N08: this parameter is a plain bool, so a call like Save(true) says nothing at the call site — define a named bool type (type SyncMode bool) with named constants, so the call reads Save(SyncImmediate)
```

The compliant rewrite:

```go
type SyncMode bool

const (
	SyncImmediate SyncMode = true
	SyncDeferred  SyncMode = false
)

func save(mode SyncMode) {
	_ = mode
}
```

**Known miss:** Function literals are exempt, for the same reason as TS-N07. Their
signatures are often dictated by the API that consumes them, so a finding there would be
unfixable at the literal itself.

<!-- rule: TS-N14 -->
### TS-N14 — exported identifiers do not end in a present participle

**Severity:** blocking — a finding fails the run.

A name ending in "-ing", like `Preparing` or `Loading`, describes an activity instead of
a thing. Nouns compose better. `Pipeline` can take a suffix like `pipelineMax`, while
`preparing` cannot extend the same way. Rename the identifier to the noun it stands for.

Code that fires it:

```go
type Preparing struct{}
```

```
state.go:1:6: TS-N14: "Preparing" ends in the -ing word "Preparing", which names an activity rather than a thing — rename it to the noun it stands for, or pass the flag -allow=preparing if it really is a noun
```

The compliant rewrite:

```go
type Preparation struct{}
```

**Known miss:** The analyzer walks types, functions, methods, consts, vars, and struct
fields. Interface methods are not in that set, so an exported interface method ending in
a participle stays silent.


## Tests

<!-- rule: TS-T02 -->
### TS-T02 — map iteration order never reaches an output

**Severity:** blocking — a finding fails the run.

Go deliberately randomizes the order it visits map entries in, on every single run. If a
loop appends, builds a string, or sends those entries somewhere, that randomness leaks
into the output. The fix is to collect and sort the keys first, then range over the
sorted slice.

Code that fires it:

```go
func Names(m map[string]int) []string {
	var result []string
	for k := range m {
		result = append(result, k)
	}
	return result
}
```

```
names.go:5:2: TS-T02: this loop appends to a slice while ranging over a map, and Go visits map entries in a different order every run, so the slice's order varies — range over the sorted keys instead: for _, k := range slices.Sorted(maps.Keys(m))
```

The compliant rewrite:

```go
func Names(m map[string]int) []string {
	var result []string
	for _, k := range slices.Sorted(maps.Keys(m)) {
		result = append(result, k)
	}
	return result
}
```

The sorted rewrite is not the only shape that passes. A map loop that appends into a local
slice passes when the slice's first use after the loop is `sort.Strings`, `sort.Ints`, or
`slices.Sort` on string or integer elements, since equal strings or integers cannot be told
apart:

```go
ids := make([]string, 0, len(m))
for id := range m {
	ids = append(ids, id)
}
sort.Strings(ids)
```

Floats do not pass, because -0 and +0 compare equal and print differently. A comparator sort
(`slices.SortFunc`, `slices.SortStableFunc`, `slices.SortedFunc`) passes only when its comparisons
together cover every field of the element, using `cmp.Compare`, `strings.Compare`, or
`bytes.Compare` on a whole array. Pointer, interface, bool, float, and `time.Time` fields never
count as covered. A key type's `Compare` method that passes the same check counts, as in
`slices.SortedFunc(maps.Keys(m), K.Compare)`. Any other comparator sort fires, and its message
names `slices.Sorted(maps.Keys(m))` followed by `slices.SortStableFunc`.

A result of `maps.Keys`, `maps.Values`, or `maps.All` fires unless it goes to `slices.Sorted`, a
full-coverage `SortedFunc` or `SortedStableFunc`, `maps.Collect`, `maps.Insert`, or a range checked like a map range, so
`slices.Collect(maps.Keys(m))` is a finding. A map write gated by a counter, such as copying the
first three entries visited, fires too.

A finding means the loop's shape is outside the allowlist, not that the output varies.

**Known misses:** Five shapes pass although they depend on order. Last writer wins
(`inverse[v] = k` when two keys share a value). A map write whose value calls a function
(`labels[k] = fmt.Sprint(v)`). `golang.org/x/exp/maps.Keys`. `reflect.Value.MapKeys`. A range over
a type parameter constrained to a map. TS-T11, which runs the tests twice and compares the output,
can catch these on the paths the tests exercise, though two runs sometimes visit entries in the same
order.

<!-- rule: TS-T06 -->
### TS-T06 — test functions have a doc comment

**Severity:** blocking — a finding fails the run.

A test with no doc comment forces a reader to reconstruct its purpose from the setup
code alone. That is the wrong moment for detective work, since the reader is usually
already debugging a failure. A one-line comment above the test lets a reader decide to
skip it or dive in immediately.

Code that fires it:

```go
func TestNoDocComment(t *testing.T) {
	_ = t
}
```

```
handler_test.go:5:1: TS-T06: TestNoDocComment has no doc comment — add one above it saying what the test proves, so a reader can decide to skip it or dig in without reverse-engineering the setup
```

The compliant rewrite:

```go
// TestHandlesEmptyBody proves the handler returns 400 when the request body is empty.
func TestHandlesEmptyBody(t *testing.T) {
	_ = t
}
```

**Known miss:** Benchmark and fuzz functions are not checked yet. Only functions shaped
like `func TestXxx(t *testing.T)` are.

<!-- rule: TS-T10 -->
### TS-T10 — table-driven tests have named cases

**Severity:** blocking — a finding fails the run.

A table-driven test with no name field fails as a bare index number. When a test breaks
at 2am, that index tells the reader nothing about what was being tested. Adding a name
field means the failure output names the case directly, right where the reader is
already standing.

Code that fires it:

```go
func TestSum(t *testing.T) {
	cases := []struct {
		input int
		want  int
	}{
		{input: 1, want: 1},
	}
	for _, test := range cases {
		if test.input != test.want {
			t.Fatalf("got %d, want %d", test.input, test.want)
		}
	}
}
```

```
sum_test.go:6:11: TS-T10: this table-driven test's case struct has no name field, so a failing case can't say which one it was — add a name string (or Name string) field
```

The compliant rewrite:

```go
func TestSum(t *testing.T) {
	cases := []struct {
		name  string
		input int
		want  int
	}{
		{name: "one", input: 1, want: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.input != test.want {
				t.Fatalf("got %d, want %d", test.input, test.want)
			}
		})
	}
}
```

**Known miss:** A named struct type used as the case type, `[]sumCase` instead of an
inline `[]struct{...}`, is not checked even with no name field.


## Effects and frames

<!-- rule: TS-F01 -->
### TS-F01 — an effects pin is an exact, bidirectional contract

**Severity:** blocking — a finding fails the run.

A `//tiger:effects` pin is a two-way promise. Nothing undeclared may happen, and nothing
declared may be missing. Tiger computes a function's real effects (network calls, disk
access, allocation, panics) across its whole call chain, which catches a behavior hiding
behind a helper three calls deep that a name-only check would miss.

Code that fires it:

```go
//tiger:effects none
func Grow() []int {
	return make([]int, 4)
}
```

```
ring.go:3:1: TS-F01: this function allocates (heap allocation at ring.go:5:13) but its //tiger:effects comment doesn't list alloc — add alloc to the comment, or remove that code
```

The compliant rewrite:

```go
//tiger:effects alloc
func Grow() []int {
	return make([]int, 4)
}
```

**Directives:** `//tiger:effects <set>` pins a function's effects (for example
`//tiger:effects io(net)`, or `//tiger:effects none` for purity). The pin is exact both
ways. An undeclared effect fails, and a declared-but-absent effect fails too. Absence of
a pin is never a finding; `tiger pin <Name>` writes one for you.

**Known miss:** Tiger follows calls it can resolve statically. What a stored function
value (a callback, an injected dependency) does never joins the computed effect set, so
a pin can pass even though the callback does something the pin does not declare.

<!-- rule: TS-F02 -->
### TS-F02 — a pin bounds the entire subtree beneath it

**Severity:** blocking — a finding fails the run.

A pin covers everything a function calls, not just its own body. If any function beneath
it starts doing something new, the pin fails and names the call that introduced it.
Pinning one exported function locks down every private helper it calls, directly or
indirectly, so sparse pins still give dense enforcement.

Code that fires it:

```go
//tiger:effects none
func Serve() {
	dial()
}

func dial() {
	net.Dial("tcp", "localhost:0")
}
```

```
logger.go:5:1: TS-F02: this function makes a network call (app.example/logger.dial at logger.go:7:6) but its //tiger:effects comment doesn't list io(net) — add io(net) to the comment, or remove the call
```

The compliant rewrite:

```go
//tiger:effects io(net)
func Serve() {
	dial()
}

func dial() {
	net.Dial("tcp", "localhost:0")
}
```

**Directives:** The same `//tiger:effects <set>` pin as TS-F01, exact both ways.
Absence of a pin is never a finding; `tiger pin <Name>` writes one.

<!-- rule: TS-F07 -->
### TS-F07 — writes outside a pinned frame fail, bidirectionally

**Severity:** blocking — a finding fails the run.

A `//tiger:frame` pin lists every location a function is allowed to write. Tiger
computes the real set, every location reachable from the function's parameters and
receiver that the function writes. A pinned frame answers "which function could have
written this value" with one search instead of a read of the whole program.

Code that fires it:

```go
type Recorder struct {
	log        string
	checkpoint string
}

//tiger:frame r.log
func (r *Recorder) WriteBoth(msg string) {
	r.log = msg
	r.checkpoint = msg
}
```

```
recorder.go:8:1: TS-F07: this function writes r.checkpoint (at recorder.go:11:4) but its //tiger:frame comment doesn't list it — add r.checkpoint to the comment (//tiger:frame r.checkpoint, r.log), or remove the write
```

The compliant rewrite:

```go
//tiger:frame r.log, r.checkpoint
func (r *Recorder) WriteBoth(msg string) {
	r.log = msg
	r.checkpoint = msg
}
```

**Directives:** `//tiger:frame <locations>` pins which locations a function may write,
exact both ways. A write outside the list fails, and a listed location the function
never touches fails too. Absence of a pin is never a finding; `tiger pin <Name>` writes
one.

**Known miss:** Tiger traces a write back to the function's own parameters and receiver.
When a helper returns a pointer into that state instead of taking it as a parameter, a
write through the returned pointer never joins the computed frame, so a pin can look
exact while the function quietly writes something the pin never mentions.


## Termination and contracts

<!-- rule: TS-V01 -->
### TS-V01 — every unbounded loop has a verified variant

**Severity:** blocking — a finding fails the run.

A loop needs proof it actually ends, not just a condition that looks like it shrinks. A
variant is a value that shrinks every pass and has a floor it cannot cross, the standard
argument that a loop terminates. Tiger finds this proof automatically for most loops.
Where it cannot, you write the shrinking expression yourself, and tiger checks that it
holds.

A loop that fires it:

```go
func drainConditional(pending []int, extra func() bool) int {
	drained := 0
	for len(pending) > 0 {
		if extra() {
			pending = pending[1:]
		}
		drained++
	}
	return drained
}
```

```
drain.go:5:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
```

The compliant rewrite:

```go
func drain(pending []int) int {
	drained := 0
	for len(pending) > 0 {
		pending = pending[1:]
		drained++
	}
	return drained
}
```

Tiger finds a shrinking expression for these shapes:

- a fixed step, such as `i--`, `i += 2`, or `s = s[1:]`
- division by a constant greater than 1, or a shift by a constant of at least 1, under a
  strict `> 0` condition, as in `for size > 0 { size >>= 7 }`
- slicing by the length of a part tiger can show is not empty, as in
  `data = data[len(chunk):]` where `chunk` is `data` or `data[:max]` with `max` a
  constant above 0, and `chunk` is not reassigned before the slice
- a struct field in the condition, such as `len(q.Items) > 0`, when no call in the body
  receives `q` or a pointer to it
- a condition joined with `&&` where one part shrinks, as in
  `for x > 0 && y > 0 { x--; y-- }`; a negated condition does not count

A proof is sound. Variables are matched by identity, not by name, so a shadowing copy does
not count. Growth through a pointer or a closure made before the loop defeats a proof,
and so does a counter that wraps around at the end of its type (`i <= math.MaxInt`). A
`continue`, a labeled `continue`, or a `goto` that can skip the shrink defeats it too.
TS-S02 accepts a loop whose variant this rule verified.

A `//tiger:variant` comment on a loop that ends on its own, a range or a counted loop,
is a finding too, because it says nothing: `TS-V01: this loop has a //tiger:variant
comment but ends on its own (a range or counted loop), so the comment says nothing —
remove it`.

**Directives:** This rule fires only where tiger cannot synthesize a shrinking
expression on its own; most loops need no pin at all. Where synthesis fails,
`//tiger:variant <expr>` pins the expression to check. The variant pin attaches to the
loop itself, wherever it lives.

<!-- rule: TS-V03 -->
### TS-V03 — preconditions are declared and discharged at call sites

**Severity:** blocking — a finding fails the run.

Checking a precondition only inside the called function means the bug surfaces at
runtime, maybe in production. Declaring the precondition lets tiger check each call
site instead, before the code ever runs. The declaration travels with the function, not
just inside its one careful caller.

Code that fires it:

```go
//tiger:requires p != nil
func needsPointer(p *int) int {
	return *p
}

func broken() int {
	return needsPointer(nil)
}
```

```
header.go:9:9: TS-V03: this call violates //tiger:requires p != nil on the callee — the argument is the literal nil — pass a value that satisfies the condition, or check it before the call
```

The compliant rewrite:

```go
//tiger:requires p != nil
func needsPointer(p *int) int {
	return *p
}

func fixed() int {
	x := 5
	return needsPointer(&x)
}
```

**Directives:** `//tiger:requires <cond>` pins a precondition the caller must establish
before the call; `//tiger:ensures <cond>` pins a postcondition the function guarantees
on return.

**Known miss:** Tiger proves a violation only from a literal value at the call site, a
bare nil or a constant out of range. When the bad value comes from earlier code, tiger
stays silent and the check falls back to the callee's own runtime assertion.


## Invariants

<!-- rule: TS-A07 -->
### TS-A07 — every invariant is asserted in a function outside _test.go files

**Severity:** blocking — a finding fails the run.

An invariant is a property you name as a constant ID and check with `assert.Invariant`.
A declared invariant that nothing ever asserts is a claim in the code that nothing
checks. One production call site is enough. Evidence for this rule spans packages that
do not import each other, so only the `tiger` CLI reports it; the golangci-lint plugin
cannot.

Code that fires it (`inv` declares two IDs; production code asserts only one):

```go
// inv/consts.go
package inv

type ID string

const (
	HeaderSize     ID = "header-size"
	HeaderChecksum ID = "header-checksum"
)
```

```go
// wire/wire.go
func encodeHeader(target []byte, size int) {
	assert.Invariant(inv.HeaderSize, len(target) == size)
}
```

```
consts.go:7:2: TS-A07: invariant inv.HeaderChecksum is declared but no function outside _test.go files asserts it — add assert.Invariant(inv.HeaderChecksum, ...) where the property is established, or delete the declaration
```

The compliant rewrite asserts both where the properties are established.

```go
// wire/wire.go
func encodeHeader(target []byte, size, checksum int) {
	assert.Invariant(inv.HeaderSize, len(target) == size)
	assert.Invariant(inv.HeaderChecksum, checksum != 0)
}
```

**Directives:** The invariant vocabulary is a package-level const of a named string
type, used as the ID in an `assert.Invariant` or `assert.Violates` call. Tiger
recognizes it by that shape, not by a special import.

<!-- rule: TS-A09 -->
### TS-A09 — every invariant has a test that violates it

**Severity:** blocking — a finding fails the run.

Every invariant needs a test that proves it can actually fail. If no test can trigger
the failure, one of two things is true. Either the property can never be reached, or the
check itself is broken. Evidence for this rule spans packages that do not import each
other, so only the `tiger` CLI reports it; the golangci-lint plugin cannot.

Code that fires it (the invariant is asserted in production, but no test violates it):

```go
// inv/consts.go
package inv

type ID string

const HeaderChecksum ID = "header-checksum"
```

```go
// wire/wire.go
func encodeHeader(checksum int) {
	assert.Invariant(inv.HeaderChecksum, checksum != 0)
}
```

```
consts.go:5:7: TS-A09: no test violates invariant inv.HeaderChecksum — add a _test.go function that calls assert.Violates(inv.HeaderChecksum, func() { ... })
```

The compliant rewrite adds the violation test.

```go
// wire/wire_test.go
func TestHeaderChecksumViolates(t *testing.T) {
	assert.Violates(inv.HeaderChecksum, func() {
		encodeHeader(0)
	})
}
```

**Directives:** The same invariant vocabulary as TS-A07.

## Budget accounting

<!-- rule: TS-D06 -->
### TS-D06 — per-package budgets may only decrease

**Severity:** blocking — a finding fails the run.

`tiger.budget.yaml` records how many advisory findings (skipped tests, escape
directives) each package may carry. `tiger budget --write` can only lower those numbers
to match the current count, create a missing row, or delete one that reaches zero. It
has no operation that raises a number. Raising one is a hand edit a reviewer sees in a
pull request, so nobody can quietly relax a budget at 2am before a release.

A budget row that is too low:

```yaml
internal/cli:
  TS-D07: 2
```

```
tiger.budget.yaml:2: TS-D06: internal/cli has 4 skipped tests but tiger.budget.yaml allows 2 — fix 2 of them, or raise the number in a reviewed edit
internal/cli/skip_test.go:9:2: TS-D07: this test is skipped, so it passes without running — remove the Skip call when the test can run again; this notice stands until then
```

The two exits are exactly what the line says. Fix enough findings to get back under the
row, or raise the row by hand and let the reviewer rule on it. A package with counted
findings and no row at all has a budget of zero, and the finding prints without a line
number.

```
tiger.budget.yaml: TS-D06: internal/driver has 1 skipped test and tiger.budget.yaml has no row for it — fix it, or run tiger budget --write to record the current count
```

<!-- rule: TS-D07 -->
### TS-D07 — skipped tests are counted against the package budget

**Severity:** advisory — findings are counted against the package's row in tiger.budget.yaml: silent under budget; on overrun they print as blocking lines under a TS-D06 line. See [Budgets and the ratchet](#budgets-and-the-ratchet).

A skipped test counts as passing even though it never ran, which can hide a real
problem. Tracking each skip against the package's budget keeps it visible on every run.
That is better than a comment pointing at a tracker issue, because the count is
recomputed from the code each time and cannot go stale.

Code that fires it:

```go
func TestReplication(t *testing.T) {
	t.Skip()
}
```

```
skip_test.go:2:2: TS-D07: this test is skipped, so it passes without running — remove the Skip call when the test can run again; this notice stands until then
```

The compliant rewrite removes the skip so the test runs again.

```go
func TestReplication(t *testing.T) {
	require.Equal(t, 3, replicaCount(cluster))
}
```

**Known miss:** The analyzer resolves a Skip call by its receiver's immediate static
type. A Skip reached through a struct that embeds `*testing.T` stays silent, because the
receiver's static type is the wrapping struct.


## Auto rules

Tiger does not reimplement what an off-the-shelf linter already enforces well. These rules
are the auto half of the rule set. Each one is delegated to a golangci-lint linter, and
`tiger golangci` audits your golangci-lint config against this exact baseline (generate it
with `tiger golangci --init`, print it with `--print`). Every auto rule blocks, because
golangci-lint findings fail its run.

| Rule | Requirement | Linter |
|---|---|---|
| TS-S04 | hard limit of 70 lines per function | funlen |
| TS-S05 | cyclomatic complexity at most 10 | cyclop |
| TS-S05 | cognitive complexity at most 15 | gocognit |
| TS-S05 | nesting depth at most 3 | nestif |
| TS-S08 | every switch over a fixed set of values is exhaustive | exhaustive |
| TS-S10 | no init() functions | gochecknoinits |
| TS-S11 | no package-level mutable state | gochecknoglobals |
| TS-S12 | no unsafe, no cgo, no reflection | depguard |
| TS-S13 | no dead assignments | ineffassign |
| TS-S13 | no wasted assignments | wastedassign |
| TS-S14 | no shadowing | govet |
| TS-S15 | struct literals name every field on config and wire types | exhaustruct |
| TS-S16 | no magic numbers | mnd |
| TS-S19 | integer conversions are checked | gosec |
| TS-S19 | no pointless conversions | unconvert |
| TS-S20 | type assertions are always checked | forcetypeassert |
| TS-S20 | type assertions are checked via errcheck | errcheck |
| TS-E01 | every error is handled | errcheck |
| TS-E03 | wrap once with %w, compare with errors.Is and errors.As | errorlint |
| TS-E03 | wrap errors crossing package boundaries | wrapcheck |
| TS-E04 | sentinel errors are ErrFoo, error types are FooError | errname |
| TS-E04 | error strings are lowercase and unpunctuated | staticcheck |
| TS-E05 | never return both a nil value and a nil error | nilnil |
| TS-E06 | accept interfaces, return concrete types | ireturn |
| TS-E08 | no naked returns | nakedret |
| TS-C07 | context is never stored in a struct | containedctx |
| TS-C07 | context propagates | contextcheck |
| TS-C07 | no HTTP without context | noctx |
| TS-C08 | no time.Sleep in production code | forbidigo |
| TS-M01 | preallocate with a known capacity | prealloc |
| TS-M01 | no make-plus-append misuse | makezero |
| TS-M03 | pass structs larger than 64 bytes by pointer | gocritic |
| TS-M07 | strconv over fmt | perfsprint |
| TS-N01 | MixedCaps; acronyms keep their case | revive |
| TS-N02 | no invented abbreviations | varnamelen |
| TS-N10 | no stutter | revive |
| TS-L01 | gofmt is not negotiable; gofumpt on top | gofumpt |
| TS-L02 | lines at most 100 columns | lll |
| TS-L03 | declaration order is const, var, type, func | decorder |
| TS-L06 | comments are sentences | godot |
| TS-L07 | every exported identifier has a doc comment | revive |
| TS-L09 | every suppression carries a reason | nolintlint |
| TS-L11 | imports grouped standard, external, internal | gci |
| TS-T01 | time and randomness are injected | forbidigo |
| TS-D01 | the standard library, and nothing else, in internal/domain packages | depguard |

## Computed facts

Three things tiger computes are not rules. They carry no severity, never count, and print
only when you ask with `tiger check --show-facts`. Each prints in pin syntax, ready to
freeze with `tiger pin`.

| Fact | What it is | Pin it freezes |
|---|---|---|
| effect set | what a function does (allocate, do IO, block, panic, read time or randomness, mutate, spawn), including everything reachable beneath it | `//tiger:effects` (TS-F01, TS-F02) |
| frame | which locations reachable from parameters and receiver a function writes | `//tiger:frame` (TS-F07) |
| variant | the value that shrinks every iteration and proves a loop ends | `//tiger:variant` (TS-V01) |

An unpinned fact is just the current value. Pin it and drift becomes a blocking finding.
That is the change-detection half of tiger. A silent edit to what a function does cannot
land without either updating the pin (visible in review) or failing the run.
