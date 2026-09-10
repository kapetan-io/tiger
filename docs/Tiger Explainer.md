# Tiger, explained

> Tiger deterministically forces AI agents to write Go that is testable and free of whole
> classes of production bugs. It holds the code to the restrictions NASA's Power of Ten and
> TigerBeetle's TigerStyle put on flight and database software. Every loop has a bound,
> every goroutine has an owner, and every clock, random source, and IO call is passed in,
> so any function can be tested in isolation when you need to. Tiger has no warnings: the
> code passes static analysis, or the agent must rewrite it. No LLM is involved. Every
> verdict comes from deterministic static analysis.

`tiger check` · binary verdict · change detection

**This page describes tiger as it is meant to be when it is finished.** Every check it
describes is built, and every transcript on the page is real `tiger check` output. Where
a section says what we require of code rather than what tiger checks, it says so. The
rule reference lists every rule tiger enforces today, and the specification marks each
rule that is not yet built.

## Your agent writes faster than you can read

Your coding agents produce diffs faster than you can read them. Tiger holds the code written by AI to a strict set of proven restrictions, restrictions that make every source of nondeterminism injectable and shut off whole classes of bugs. Tiger takes its name from TigerStyle.

Each rule in tiger bans a construct that compiles, reads as idiomatic Go, and fails in production under an input or a timing nobody tested. For example:
- A loop with no provable bound is a denial of service waiting to happen.
- A goroutine with no owner outlives shutdown and leaks.
- A `time.Now()` buried deep in your business logic reads a different time on every run, so a failure you saw once can never be reproduced in a test.

Tiger enforces each rule with its own analyzer, so conformance is not a guess an AI reviewer can make. It forces the AI to write safe, testable code.

With tiger there is no `//nolint` equivalent to silence a finding, and no warnings to scroll past. A finding either blocks the build or tiger doesn't report it, so the agent can't bypass a check. The AI is not allowed to write untestable, buggy slop.

Tiger's second job is change detection. While it checks, tiger computes each function's **effects**, the actions it performs anywhere beneath it (allocate, do IO, block, read the clock, spawn a goroutine). Tiger lets you freeze a function's effects into a comment called a pin. Any change that silently breaks the promise fails the build, so a review sees every change to a pinned function's effects. An agent can't drift a pinned fact past you.

### Benefits of using Tiger

**Tiger makes nondeterminism injectable**

- **Failures you can reproduce.** When a test supplies the clock and the store, the
  same inputs give the same run, so a bug the test saw once it sees again.
- **Tests that cannot flake on timing.** A test that controls the clock has no timing
  to fail on.
- **Agents stop burning tokens on reachability.** In a run the test controls, whether a
  state can be reached has an answer, so the agent goes to the fix instead of
  reasoning about interleavings.
- **You can test the failures you can't provoke.** A clock, network, disk, or store
  that arrives through an interface can be replaced by a test double, a stand-in used
  only in tests, that produces a timeout or a torn write on demand, where you decide a
  double is worth writing.

**Tiger restricts the language**

- **Whole bug classes can't be written.** Unbounded loops, leaked goroutines,
  dropped switch cases, uncancellable waits, all excluded.

**Tiger lets you declare effects**

- **You find out on the commit, not in production.** An effect the code shouldn't
  have fails CI on the commit that introduced it.
- **Effect promises can't go stale.** A pinned effect that's wrong is a failed
  check, so a comment and the code it describes can't drift apart.
- **You catch what you didn't read.** An agent adds an effect deep in the tree that
  the pin above it doesn't declare, and the check fails at the pin. You never had
  to see that line of the diff.
- **A few pins cover the whole tree.** One pin on an exported function holds every
  helper beneath it, across packages, to the same promise.

**Tiger has no warnings**

- **The agent can't bypass the check.** A run fails or it passes. Tiger has no
  warnings and no suppression comment, so the agent's only path forward is a
  rewrite.
- **Review shrinks to the declarations.** If a change passes the check and edits no
  pin, restriction, or invariant, nothing in it is left for a human to review,
  because the machine already answered every question a reviewer would ask.
- **Trust attaches to the code, not the author.** Machine-written and hand-written
  code pass through the same gate and earn the same standing.

## Tiger is not a linter

Tiger and golangci-lint do different jobs, and a Tiger Go project runs both. A linter
looks for likely problems and prints warnings. A warning asks a human to decide, and
a human under deadline decides to scroll past; that's how a long-lived codebase ends
up with a warning count nobody reads anymore. That isn't a failure of any particular
linter. It's the nature of advisory output.

Tiger's output is never advisory. It checks whether your code follows the rules,
and what it flags are the patterns that turn into subtle bugs, the unbounded loop,
the switch that silently drops a case, the wait that can't be cancelled. Every
finding is a verdict that fails the build.

Tiger works with golangci-lint to keep catching likely bugs and style
issues, and tiger configures it for you (`tiger golangci --init` generates the
config for the rules existing linters already enforce well, so tiger doesn't
reimplement them).

We chose every rule for decidability, not taste. Each one
exists to make some analysis possible. A bounded loop makes termination checkable, a
supervised goroutine makes shutdown traceable, and a package that declares a
restriction like `no-reflect` (no reflection anywhere in the package) hands the
analyzers a smaller world to reason about. No single restriction is worth much on
its own, and completing a chain of them turns an analysis that's intractable on
ordinary Go into a cheap one tiger runs on every commit.

## Using Tiger
You write code → `tiger check` → you fix the findings → you pin what matters
### 1. Run the check

Say you write the following. It compiles, it's idiomatic Go, and reviewers wave it through
every day.

```go
// Drain consumes entries until none remain.
func Drain(entries <-chan uint32) uint32 {
    var total uint32
    for entry := range entries {
        total += entry
    }
    return total
}
```

```
$ tiger check ./...
drain.go:6:2: TS-S02: this loop ranges over a channel, so it ends only when some other goroutine closes the channel — add a counter cap that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
tiger: 1 blocking
```

The finding line names the rule (`TS-S02`), says what the code actually does, and names two ways to fix it.
### 2. Pin what matters

While checking, tiger also computes facts about your functions, what effects they
have (allocate, do IO, block, panic, read time or randomness, mutate, spawn), and
what they write through their parameters, which tiger calls the function's frame.
Facts aren't findings. They print only when you ask.

```
$ tiger check --show-facts ./...
header.go:34:1: TS-F01: EncodeHeader's effects are alloc — nothing to fix; to make tiger fail the build if they change, add //tiger:effects alloc
```

The fact prints as the exact comment you'd paste. Freezing it is called **pinning**,
and `tiger pin` writes the comment for you.

```
$ tiger pin EncodeHeader
header.go:35: //tiger:effects alloc
header.go:36: //tiger:frame none
```

A pin changes nothing at run time. It changes what CI enforces. Suppose someone (or
some agent) later makes a pinned function block on a channel. Here `Drain`, pinned with
no `block` among its effects, gains a channel receive. The next check fails.

```
drain.go:5:1: TS-F01: this function blocks (channel receive at drain.go:8:2) but its //tiger:effects comment doesn't list block — add block to the comment, or remove that code
```

There are two ways to pass this check: change the code back, or change the pin. A pin edit is visible in the diff, so the reviewer rules on the new promise. A pin can never go quietly stale, because a wrong pin is a failed check, not a lie in a comment.

| Condition                               | What tiger does                          | Merge                   |
| --------------------------------------- | ---------------------------------------- | ----------------------- |
| No pin; the function's behavior changes | Nothing (the fact just has a new value)  | Continues               |
| Pin; pin and code agree                 | Silence                                  | Continues               |
| Pin; pin and code disagree              | Blocking finding with the reason         | **Stops**               |
| You edit a pin                          | Nothing; the edit is visible in the diff | Continues, after review |

## Key Concepts in Tiger

### What a surface is
A surface is where your code meets something it doesn't control. Most codebases have two surfaces: one inbound, the exported functions and methods callers reach, and one outbound, the functions your code calls to reach external systems.

Inbound Surface
The inbound surface is the exported functions and methods through which callers you
don't control reach your code. This is where you pin a function, freezing what tiger has computed about it.

Outbound Surface
The outbound surface is where your code reaches the world it doesn't control, like the disk, the clock, the network, or anything it reaches through an interface it was handed.

Consider this example which identifies both the outbound and inbound surfaces.
```go
// Clock is an outbound surface. Production hands in the wall clock and a
// test hands in one it controls.
type Clock interface {
    Now() time.Time
}

// Expire is on the inbound surface. It reads time only through the clock
// it was given.
func Expire(clock Clock, entries []Entry) []Entry {
```

Tiger checks both surfaces through the same fact. On the inbound surface it computes
every exported function's effect set, the list of what the function does besides return
its result, and once you pin a function it holds the function to that list. On the
outbound surface, whatever a function reaches directly, the clock, the disk, the
network, shows up in that list, and whatever it reaches through an interface it was
handed does not. So a pin on an exported function is a promise about what that function
reaches on the outbound surface, however many helpers sit between the two, and a pin
that lists no clock, randomness, or IO is a proof that all of it arrives by injection.

### Nondeterminism is injectable, and a pin proves it

A function is deterministic when the same inputs always produce the same result.
Nondeterminism is the opposite: the same call can return a different result from one
run to the next. That happens when the function reads something outside its inputs, and
the things a function reads that way are the clock, a random number, a fresh ID, and the
network. A run that reads one of those cannot be repeated
exactly, so a failure it produced cannot be reproduced.

Our rule is that every such input is injectable. A function that needs the clock,
randomness, or IO takes it as an interface parameter, so a caller can hand in the real
thing in production and something else in a test. We do not require the something
else. Whether a test substitutes a dependency or uses the real one is a judgment the
coder makes per test, and for a crypto library the real one is usually the better
choice. What we require is that the choice exists.

Tiger checks that through the effect set. A call made through an interface adds
nothing to a function's effects, because tiger cannot see which implementation will
answer. A direct call adds one: `time` for the clock, `rand` for randomness, `io` for
the disk or the network. So a pinned function whose effects list none of those is a
function whose nondeterminism all arrives by injection, and a commit that adds a direct
read beneath it fails at the pin. Here `Expire` reads the clock through the interface
it was given and `ExpireDirect` reads it itself, and both are pinned as having no
effects.

```go
type Clock interface {
	Now() time.Time
}

//tiger:effects none
func Expire(clock Clock, deadline time.Time) bool {
	return clock.Now().After(deadline)
}

//tiger:effects none
func ExpireDirect(deadline time.Time) bool {
	return time.Now().After(deadline)
}
```

```
$ tiger check ./inject/...
inject/inject.go:22:1: TS-F02: this function reads the clock (time.Now at inject.go:24:17) but its //tiger:effects comment doesn't list time — add time to the comment, or remove the call
tiger: 1 blocking
```

`Expire` passes. `ExpireDirect` fails, and the finding names the call. Map iteration
order is the one source Go supplies without being asked, and it can't be injected, so
tiger instead stops it from reaching any output that is serialized, logged, or hashed.

What this buys is code that can be tested deterministically, which is a smaller promise
than code that is deterministic. When a test hands in a clock it controls and a store it
controls, everything behind them is a function of its inputs and of the answers those
two give. The same test then produces the same run, so a failure it finds can be
reproduced. When a test hands in the real disk, the run is as repeatable as the disk
is, and that was the coder's decision to make.

A test double is a stand-in used only in tests. Where one is worth writing, a double
that produces a timeout or a torn write on demand turns those failures into ordinary
test cases. The
interface has to be designed with those faults in mind for the double to express them.

None of this comes from Power of Ten, which has no determinism rule. It is
TigerBeetle's discipline. TigerBeetle found the bugs that made it worth imitating by
running its whole system inside a deterministic simulator, and that only works if every
source of nondeterminism is behind an interface the simulator controls. Our
contribution is to make the injectable part of that discipline checkable rather than
cultural. A direct clock read beneath a pinned function isn't a review comment. It's a
blocking finding that names the call.

### Invariants have names

An invariant is a property of the data that must hold every time the code reaches a
particular point, such as a header being exactly twelve bytes long or its checksum
matching its payload. We require each one to be declared once, by name, so that the
analyzer can count where it is checked.

The checking itself is done by assertions. An assertion is not an error. An error is an
operating condition the caller has to handle, like a file that does not exist. An
assertion failure means the code itself is wrong, and the only safe response to wrong
code is to stop before it corrupts anything.

Go has no assert statement, so we ship a small assert package with no dependencies,
meant to be copied into the project. We keep its assertions enabled in production,
because an assertion that runs only under test guards nothing else.

A project declares its invariants as constants of a string type, one constant per
property, in a single package. The ledger example calls that package `inv` and declares
two.

```go
type ID string

const (
	HeaderChecksum ID = "header-checksum"
	HeaderSize     ID = "header-size"
)
```

Each place the property must hold passes the constant to `assert.Invariant` along with
the condition. `EncodeHeader` asserts both invariants on the buffer it produces, and
`DecodeHeader` asserts the same two on the buffer it is given, so the property is checked
before the bytes go out and again when they come back. If one of the two checks is
written wrong, the other still stops a bad header. At run time the constant does
nothing more than name the invariant in the failure message. Its work is done before
then, because a named constant is something the analyzer can find every reference to,
and a string inside an assertion message is not.

We derive two rules from that. The first is that production code has to assert every
invariant the project declares, somewhere, because a declaration that nothing asserts
describes a guarantee the code does not enforce. One assertion site is enough, and only
sites outside the test files count. A declaration with none fails the build with a
finding that names the invariant and the call to add.

The second is that every declared
invariant must have a test that violates it, written with `assert.Violates`, which runs a function
and fails unless that function trips exactly the named invariant.

```go
func TestShortBufferViolatesSize(t *testing.T) {
	assert.Violates(inv.HeaderSize, func() {
		ledger.DecodeHeader(make([]byte, ledger.HeaderSizeBytes-1))
	})
}
```

Deleting that test from the ledger produces this.

```
$ tiger check ./examples/ledger/...
examples/ledger/inv/inv.go:13:2: TS-A09: no test violates invariant inv.HeaderSize — add a _test.go function that calls assert.Violates(inv.HeaderSize, func() { ... })
tiger: 1 blocking
```

We require the violating test because an invariant no test can violate is one of two
things, a property the code can never actually reach or an assertion written wrong, and
either way the project wants to know which. Both rules need to see the constant in `inv`
and the assertions and tests in the packages that import it at the same time, which is
why only the `tiger` command reports them and the golangci-lint plugin does not.

Naming the invariants costs a handful of constants and changes three things. The
constants in `inv` become an accurate list of what the system guarantees, and they stay
accurate, because deleting the last assertion of any invariant fails the build. A search
for one constant turns up every site that defends it. And an invariant added to look
thorough costs a declaration, an assertion site, and a violating test, which is more
work than finding a real one.

### Effects are the fourth oracle

An effect set is tiger's list of the kinds of things a function does besides compute
its result. It exists so that a claim like "this function never touches the disk" can
be checked by a machine instead of trusted from a comment. Tiger builds the list by
reading the function's body together with every function it calls. The entries come
from a vocabulary we fixed: allocating memory, doing IO, blocking, panicking, reading the
clock, reading randomness, mutating a named value, and spawning a goroutine.

IO comes with a qualifier naming which kind, such as `io(disk)` or `io(net)`, and a
project can add qualifiers of its own by attaching one to a surface, so that every
function reaching the store carries `io(database)`. Nobody writes any of this down. The
set is derived for every function from day one, and `--show-facts` prints it.

What makes the set worth computing is the question it can answer. Go code already has
three checks that each settle one yes-or-no question about a program, and a check of
that kind is called an oracle. The compiler is the first: it settles whether the program
is well formed and well typed, and it does so before the program runs, for every input
the program could ever receive.

A test is the second. It settles whether the program does the right thing on an input
the author wrote down, at run time whenever the suite runs, so it covers exactly the
cases someone thought of and no others.

An assertion is the third. It settles whether some property holds at one point in the
code, at run time, for whatever input actually arrived, in production as much as under
test. That is why Go's lack of an assert statement matters enough that we ship an
assert package and write our assertion rules against it.

None of the three can say what a function is allowed to do. The compiler sees no
difference between a function that reads a file and one that adds two integers, as long
as both type-check, and neither a test nor an assertion can see a code path that no
input has taken yet.

An effect set is the fourth oracle. It settles exactly that question, and it settles it
the compiler's way rather than the way a test or an assertion does: before anything
runs, for every input.

The pin is what makes it a check. A function with `//tiger:effects` above it has its
computed set compared against the comment on every run. So when a later change adds a
call that opens a socket, the check fails on that commit, at the pinned function,
without any test having to reach the new call. A function with no pin still has its set
computed, but nothing is compared and nothing prints unless the facts are asked for.
Unpinned is not a claim of purity or of anything else. Purity is a positive claim, and
it is written `//tiger:effects none`.

One thing the effect oracle leaves alone is whether the effects are appropriate. Tiger
can prove that a function blocks. We give it no opinion on whether a function in that
position ought to block, or touch the disk, or spawn anything. That judgment belongs to
whoever reviews the diff, and a pin puts it in one visible place, on the line where the
effect set is written down.

### One pin holds the whole subtree

A pin on a function is a promise about everything that function calls, not only about
its own body, and that reach is what lets a few pins on a package's exported functions
hold the whole package to a promise. The reach follows from how tiger computes an
effect set. It walks the function's body and every function reachable from it, and the
set it reports is everything it finds all the way down. So a helper several calls
beneath a function that reads the clock puts `time` in the set of every function above
it.

A pin freezes that whole-tree summary, so `//tiger:effects none` on an exported
function says that nothing beneath it allocates, blocks, or touches the outside world
either.

The consequence is that a change anywhere under a pinned function can fail the pin, even
when the pinned function's own lines are untouched. Suppose `Serve` is pinned as pure and
calls a private helper, and someone (or some agent) later makes that helper open a
network connection.

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
$ tiger check ./...
logger/logger.go:5:1: TS-F02: this function makes a network call (app.example/logger.dial at logger.go:7:6) but its //tiger:effects comment doesn't list io(net) — add io(net) to the comment, or remove the call
tiger: 1 blocking
```

The finding is reported at the pinned function, not at the line that changed. The change
was in `dial`, but `dial` made no promise. `Serve` did, and what tiger checks is whether
`Serve` still keeps it. The call the finding names is always one in `Serve`'s own body,
which here is `dial`. If `Serve` called a helper named `prepare` and `prepare` called
`dial`, the finding would name `prepare` instead, because `prepare` is the call in
`Serve` that the effect comes through, and the path to the cause continues from there.
The fix is one of the two the finding offers: remove the network call, or change the pin
to `//tiger:effects io(net)`. As before, the pin edit is what the reviewer rules on.

The check is exact in both directions. If a later change removes the network call, an
`io(net)` pin fails the same way the `none` pin did, because the code no longer does
what the pin says. A pin cannot describe more than the code does without the check
failing, so it never drifts into a superset that means nothing.

This is why we attach pins to exported functions only. Private helpers are the code that
gets split, merged, and renamed during a refactor, and a pin on each of them would have
to be edited along with every reshaping. Nothing is lost by leaving them unpinned,
because every helper is already inside the subtree of the exported function that calls
it.

The subtree does not stop at the package boundary. A pinned function that calls into
another package is held to whatever that package's code does, and the finding names the
imported function the same way it names a private helper. Where the callee is itself
pinned, tiger reads the callee's pin instead of walking its body again, because the
callee's own check already proves that pin's accuracy, so checking stays modular exactly
where pins are dense.

### A frame is where a function writes

A function's frame is the set of locations it writes through its receiver and its
parameters, counting the writes made by anything it calls. Tiger computes the frame for
every function, without any annotation, alongside the effect set, and the two answer
different questions. The effect set says what kinds of thing a function does, such as
allocate or read the clock. The frame says which state it changes.

The frame narrows the search for a bad value. When a field holds something it should
not, the functions that could have put it there are the ones whose frame includes that
field. A function whose frame leaves the field out could not have written it, no matter
how much of the program that function touches. Nobody reads the program to find the writer; the
computed frames already say who is a candidate.

A frame pin is held exact in both directions, the same as an effects pin. Here `Apply`
is pinned to `r.log` and calls a helper that also advances the checkpoint.

```go
//tiger:frame r.log
func (r *Replica) Apply(entries []string) {
	for i := 0; i < len(entries); i++ {
		r.log = append(r.log, entries[i])
	}
	r.advance(len(entries))
}

func (r *Replica) advance(n int) {
	r.checkpoint += n
}
```

```
$ tiger check ./frame/...
frame/replica.go:9:1: TS-F07: this function writes r.checkpoint (at replica.go:14:11) but its //tiger:frame comment doesn't list it — add r.checkpoint to the comment (//tiger:frame r.checkpoint, r.log), or remove the write
tiger: 1 blocking
```

The finding is reported at the pinned function, and the position it names is the call
to `advance` in `Apply`'s own body, the same convention the effects check uses. The
other direction fails the same way. Listing `r.pending`, a field `Apply` never touches,
produces a finding that reads "this function never writes r.pending, yet its
//tiger:frame comment lists it" and prints the corrected comment.

As with effects, a pin goes only on an exported function or method, and an exported
function without one has its frame printed under `--show-facts` as the comment that
would freeze it. `tiger pin` writes that comment, which is why the `EncodeHeader` run
earlier on this page produced a `//tiger:frame none` line next to the effects line.

Tiger follows a write back to a receiver or parameter through field selections, element
addresses, and pointer dereferences, and no further. A helper that hands back a pointer
into the receiver, or code that copies part of it into a package variable, breaks that
trail. A write made through either one stays out of the computed frame, and because
tiger does not report what it cannot trace, no finding says so. A frame pin proves what
tiger can trace, and we record this in the rule reference as a known miss.

### Whole bug classes are excluded, not caught

The benefits list says four kinds of bug are excluded rather than caught, and the
difference is where the check is applied. A linter looks at code for the pattern of a
known bug, and so it finds the instances that match its pattern and misses the rest.

We instead require a particular shape of every loop, every switch over a fixed set
of values, every `go` statement, and every blocking `select`, and the shapes we allow
are ones in which the bug cannot occur. The question tiger asks is never
whether this loop happens to hang, only whether this loop is in an allowed shape, and
that question has an answer for every loop. Four small files, each written the way a
Go programmer would write it without thinking, produce one finding apiece.

```
$ tiger check ./bugs/...
bugs/drain.go:5:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
bugs/status.go:11:2: TS-S08: this switch over Status has no default arm — add default: assert.Unreachable("Status: unhandled value") so a new Status constant fails loudly
bugs/wait.go:4:2: TS-C05: this select blocks with no case that ends the wait on shutdown — add case <-ctx.Done(): return ctx.Err()
bugs/worker.go:4:2: TS-C02: this go statement starts a goroutine nobody owns: nothing says when it exits or ties it to a context — start it through errgroup.Group.Go
tiger: 4 blocking
```

The shape we allow for a loop starts with a stated bound, meaning a condition built
from a constant, a length, or a counter, and the `Drain` transcript earlier in this
document is a loop that fails on that requirement alone. A stated bound is only a
promise that the loop could end, though. A loop can test `len(pending) > 0` and never
shorten `pending`. So tiger goes further and looks for something in the loop that
shrinks on every pass and has a floor it cannot cross, which is the standard argument
that a loop terminates. We call that measure the loop's variant.

Tiger finds one unaided for nearly every real loop, and when it does the loop is
proven, nothing prints, and the expression it found is available under `--show-facts`
as the `//tiger:variant` comment that would freeze it. The first finding
above is a loop whose shortening happens only inside an `if`, so nothing
shrinks on every pass, and the finding names the two ways out: a cap that fails when
hit, or a `//tiger:variant` pin naming what shrinks.

Switches are the second shape. A `switch` over a type with a fixed set of values must
handle each value and end in a `default` arm that calls `assert.Unreachable`. The
compiler cannot check that such a switch is complete, because an integer constant is
only a number to it. A value that arrived off the wire and was never in the set must
land somewhere that crashes, rather than fall out the bottom of the switch. Once
every switch has that arm, adding a constant breaks each switch that has not
considered it, which is the outcome we want from the rule.

A type whose set of values is
meant to grow is marked `//tiger:openenum`, and a switch over it keeps a `default` arm
as a legitimate catch-all rather than a crash.

In our trials this rule found three real bugs, two of them storage backends dropping an
action in silence.

For goroutines, the shape we allow is that each one starts through a supervisor that ties it
to a context and to code that waits for it to finish, such as `errgroup.Group.Go`.
A bare `go` statement in the middle of a function is a finding, because nothing in it
says when the goroutine exits or what stops it. Starting a goroutine also puts `spawn`
in the function's effect set, so a function several calls up whose pin does not include
`spawn` fails the check on the commit that added the goroutine, however deep it was
added. Our trials found a daemon whose `sync.WaitGroup` was never waited on, so its
`Shutdown` method could return while its HTTP goroutines were still running.

The last shape is the wait. A `select` that blocks must have a case that ends the wait
on shutdown, either `ctx.Done()` or a channel that is recognizably a shutdown signal.
A loop that runs forever must have the same case in its `select`, so that the loop can
end. Without that case, a shutdown request has no effect on the wait, so the process
stays up until something kills it, and a process that dies partway through a write
loses the write. Our trials found a queue whose
Produce, Complete, and Retry waits could not be cancelled, while its documentation
said they could.

## The concepts, in order

### Rules and the two severities

Every rule has a code and exactly one of two severities. **Blocking**
findings fail the run. **Advisory** findings are counted, not printed. There's no
warning tier, on purpose. A rule either blocks or it isn't a rule, so nothing tiger
prints can be weighed and set aside.

Advisory exists for exactly two things real codebases carry, an escape directive and
a skipped test. Both are legitimate, both are worth bounding, so tiger counts them
against a per-package budget instead of pretending they're errors.

### The budget and the ratchet

`tiger.budget.yaml` is a committed file with one number per package per advisory
rule, the count that package is allowed to carry. Under budget, nothing prints. Over
budget, the run fails and the counted findings print. `tiger budget --write` records
current counts, and it can only *lower* numbers. Raising one is a hand edit a
reviewer sees in the diff. We call this the ratchet.

### Directives

A directive is a `//tiger:` comment on the line above the thing it describes. The
vocabulary is closed and a typo is a blocking finding, so a directive can't silently
do nothing. There are three kinds.

1. **Pins** (freeze). Freeze a computed fact into a contract. `//tiger:effects`,
   `//tiger:frame`, `//tiger:variant`. Optional, and written for you by `tiger pin`.
2. **Intent declarations** (declare). State something no analyzer can compute, and
   the code is held to the statement. `//tiger:restrict` (this package forgoes a
   language feature), `//tiger:openenum` (this type's value set is expected to grow).
3. **Escape directives** (counted). Loosen one rule at one site, with a mandatory
   reason. There is exactly one, `//tiger:batched`, and every use is counted against
   the budget on every run.

The rules allow exactly one escape directive, because an agent will use any comment
that gets it past a failing check. An escape is admitted only when reality, not convenience,
demands it. Some external systems accept one item at a time, and no rewrite of your
code changes that, so `//tiger:batched <reason>` exists. A general "ignore this rule
here" directive doesn't exist and won't. The only paths are the compliant rewrite or
a counted, human-reviewed escape.

## How review divides

Once the declarations exist, review splits into two jobs that get different
treatment.

**Machines check the correspondence.** Whether the code satisfies the invariant it
names, stays inside its pinned frame, terminates, covers every case, matches its
declared effect set, and respects its package restrictions. All of it mechanical, all of it on every
commit.

**Humans and AI review the declarations.** Whether this is the invariant the protocol
needs, whether this function should be permitted to touch the disk at all, whether
this limit is true of the hardware you deploy on. Questions no analyzer can answer, and the
only ones left.

The declarations are a layer, not a region. They cut across every file, so there's no
"reviewed part" of the tree and no boundary to erode. Which checks apply to a
function follows from its computed effect set, not from where someone filed it.

Declarations are a small fraction of your lines and change far less often than the
code around them, so review attention concentrates where being wrong is most
expensive. A change that touches no declaration and passes CI is a candidate to merge
without a human reading it. A change that touches a declaration goes to someone who
knows the domain, and nothing else about it needs discussing, because the mechanical
questions have already been answered.

## What tiger will not do

No warnings. No per-site suppression comments. No "informational" output on a normal
run. When a rule's findings turned out to be noise on real code, we removed the rule
from tiger entirely rather than demote it to a warning; three naming rules died
exactly that way during the trials. A smaller set of rules people actually fix beats
a large set people learn to scroll past.

Tiger can prove the code satisfies the declarations; nothing can prove the
declarations match the world. That's exactly why the declarations are the part humans
still review.

## Try it

The fastest way to form an opinion about tiger is to point it at code you know well.

```
$ go install github.com/kapetan-io/tiger/cmd/tiger@latest
$ tiger check ./...
```

Expect findings. Our first run on the 42,000-line queue we knew and trusted produced
1,023. A wall of findings isn't tiger telling you your code is bad. It's the measured
distance between idiomatic Go and the rules. Read ten of them. If most name a hazard
you'd want fixed, gate CI on `tiger check` and work through them. If they don't, no
tool should talk you into it.

To see code that already follows every rule, read
[examples/ledger](../examples/ledger) in the repository. When a finding names a rule
you want the reasoning for, the [rule reference](Tiger%20Rule%20Reference.md) has one
entry per rule, with a firing example and the compliant rewrite. And when you want
the full normative detail, the [specification](Tiger%20Specification.md) is the
authority the reference is drawn from.

---

The trial numbers come from tiger's runs on two real codebases before the rules
settled. Every transcript on this page is live `tiger check` output. The `EncodeHeader` runs
and the invariant run are against `examples/ledger`. The two `Drain` runs, the `Serve`
and `dial` run, the `Apply` and `advance` run, and the four-file `bugs` run are against
scratch packages, since the ledger has no function with helpers beneath it and no loop,
switch, goroutine, or wait written in the shapes tiger rejects. The pinned `Drain` line
is the first of that run's two findings; the second is a finding on the same receive for
blocking outside a select. The
`Clock` and `Expire` snippet under "What a surface is" is illustrative and not in the
ledger; the `Expire` and `ExpireDirect` run is real, against a scratch package that also
declares two implementations of `Clock`, which the code block omits.
