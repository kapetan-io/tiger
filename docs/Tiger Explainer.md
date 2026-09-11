`tiger check` · binary verdict · change detection

**This page describes tiger as it is meant to be when it is finished.** Every check it
describes is built, and every transcript on the page is real `tiger check` output. Where
a section says what we require of code rather than what tiger checks, it says so. The
rule reference lists every rule tiger enforces today, and the specification marks each
rule that is not yet built.

## Overview
 Tiger deterministically forces AI agents to write Go that is testable and free of whole classes of production bugs. It holds the code to the restrictions NASA's Power of Ten and TigerBeetle's TigerStyle put on flight and database software. Every loop has a bound, every goroutine has an owner, and every clock, random source, and IO call is passed in, so any function can be tested in isolation when you need to. Tiger has no warnings, the code passes static analysis, or the agent must rewrite it. Tiger does not use LLMs for analsys, deterministic static analysis only.

With tiger there is no `//nolint` equivalent to silence a finding, so the agent can't bypass a check. Tiger ensures the AI agent is not allowed to write untestable, buggy slop.

Tiger's second job is change detection. While it checks, tiger computes each function's **effects**, the actions it performs anywhere beneath it (allocate, do IO, block, read the clock, spawn a goroutine). Tiger lets you freeze a function's effects via a comment called a pin. Any change that silently breaks the promise fails the build, so a review sees every change to a pinned function's effects. An agent can't drift a pinned fact without your knowledge.

### Benefits of using Tiger
**Tiger makes nondeterminism injectable**
- **Failures are reproducble.** When a test supplies the clock and the store, the
  same inputs give the same run.
- **Tests can't flake on timing.** A test that controls the clock has no timing
  to fail on.
- **Agents debug from a reproduction.** A test that supplies the clock and the store fails the
  same way on every run, so an agent fixes the failure it can rerun instead of reasoning
  about which timing produced it.

**Tiger restricts the language**
- **Eliminate wholte classes of bugs, security issues** Unbounded loops, leaked goroutines,
  dropped switch cases, uncancellable waits, all excluded.

**Tiger lets you declare effects**
- **An agent can't change a pinned effect quietly.** Every check recomputes a pinned
  function's effects and compares them to the pin, so a commit that adds IO or a clock
  read anywhere beneath the function fails at the pin and names the call.
- **A pin is never out of date.** A pin that disagrees with the code in either direction
  is a failed check rather than a stale comment, so every pin in a passing tree describes
  the code as it is.
- **Reviews surface effect changes, not diffs** An agent adds an effect deep in the tree that the pin above it doesn't declare, and the check fails at the pin. You never had to see that line of the diff.
- **A few pins cover the whole tree.** One pin on an exported function holds every
  helper beneath it, across packages, to the same effect promise.

**Tiger has no warnings**
- **The agent can't bypass the check.** A run fails or it passes. Tiger has no
  warnings and no suppression comment, so the agent's only path forward is a
  rewrite.
- **Review shrinks to the declarations.** If a change passes the check and edits no
  pin, restriction, or invariant, the review is concerned with intent and behavior and not code.

## Tiger is not a linter

Tiger is a rule set with two engines behind it. Roughly half the rules are enforced by
golangci-lint, through linters that already exist, and the other half by the 33
analyzers in tiger's own binary, which `tiger check` runs. We never reimplement a rule
an existing linter already enforces, so a project that adopts tiger keeps running
golangci-lint, and tiger's part in that half is to configure it and audit the result.

Among the rules golangci-lint enforces: a function is at most 70 lines (funlen) and a
line at most 100 columns (lll). A switch over a closed set of values lists every case,
and a default arm does not excuse a missing one (exhaustive). Every returned error is
handled (errcheck), and no function uses a naked return (nakedret). There are no init
functions (gochecknoinits) and no package-level mutable state (gochecknoglobals). A
domain package may not import unsafe, reflect, or math/rand (depguard), and no code
calls time.Now, time.Sleep, time.After, or panic directly (forbidigo).

`tiger golangci --init` writes a `.golangci.yml` for a new project from the rule
registry, with every linter the auto rules need enabled and every setting at the rule's
value, and it refuses to run if a config already exists. `tiger golangci` with no flag
audits an existing config against that same baseline. A required linter that is not
enabled, or a setting that is missing or differs from the baseline, is a finding that
names the rule and the change that restores it. Extra linters and settings pass. The
comparison is exact, so a stricter value fails too. Generation and audit read the same
table, so a generated config passes its own audit, and no rule enters the registry
without being both generated and audited.

After the funlen limit in a generated config is changed from 70 to 80, the audit prints
this and exits 1:

```
$ tiger golangci
TS-S04: linters.settings.funlen.lines is 80, but tiger's baseline for the rule "hard limit of 70 lines per function" is 70 — set it to 70 (tiger compares exactly, so a stricter value also fails)
tiger: 1 auto rules unenforced
```

With the value back at 70 it prints nothing and exits 0.

golangci-lint fails its run on any issue, and the generated config removes its caps on
how many issues it reports, so every issue is printed. The config also enables
nolintlint with an explanation required, so a `//nolint` on that half must name the
linter and give a reason. The custom half allows no such exception: `tiger check` has
no suppression comment, and its one escape directive, `//tiger:batched`, loosens one
rule at one site and counts against the package's budget on every run. CI runs
golangci-lint with the generated config and `tiger check ./...`, and both must pass.

The custom analyzers check what a function does, anywhere beneath it: its effects, what
it writes (its frame), whether every loop has a bound, whether every goroutine has an
owner, and whether the code matches its pins and its declared restrictions. A few rules
are whole-program, decided once every package has been visited, and only `tiger check`
reports those. The same analyzers also run as a golangci-lint plugin, minus the
whole-program rules and the budget.

We chose every rule so that a machine can answer it yes or no: "does this loop have a
bound" has an answer, and "does this look risky" does not. Each rule exists to make one
analysis possible. A bounded loop makes termination checkable, and a goroutine with an
owner makes shutdown traceable.

A package can declare restrictions in its doc comment with `//tiger:restrict`, on three
axes. no-reflect forbids reflection, so every call target is visible in the source.
closed-dispatch requires every interface call to resolve to a known set of
implementations. imports(...) limits the package to the standard library plus the
packages it names. A package with no declaration is not an error, since each axis has a
default, but a declaration the package's own code contradicts is a blocking finding.

A package's precision is bounded by the weakest package it transitively imports, because
closed dispatch in one package gives nothing if a dependency reintroduces a call whose
target is unknown. Tiger computes that bound, so a package claiming an axis its
dependencies do not support gets a blocking finding at the package clause, naming the
first dependency that weakens it. The standard library never weakens the bound, since
tiger carries a curated fact table for it.

## Using Tiger

### 1. Run the check

`tiger check` reads the packages of a module, checks them against the rules, and prints
one line for each rule the code breaks. A developer runs it as `tiger check ./...` from
the command line, as one step of four: write the code, run the check, fix what it
finds, then pin what matters, which the next subsection covers. A run that finds
nothing prints nothing and exits 0. A run that finds something prints a line per
finding and a count, and exits 1.

This function compiles, is idiomatic Go, and reviewers approve code like it every day.

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

Running `tiger check ./...` against it produces this transcript.

```
$ tiger check ./...
drain.go:6:2: TS-S02: this loop ranges over a channel, so it ends only when some other goroutine closes the channel — add a counter cap that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
tiger: 1 blocking
```

A finding line names the rule, states what the code actually does, and offers two
ways to fix it. Drain's loop ranges over a channel, so when it ends depends on some
other goroutine closing that channel. The loop itself has no way to bound that
ending. Either fix supplies the missing bound: a counter that fails once it hits a
cap, or an event loop that selects on the context so it can be canceled. Once either
fix is applied, the run prints nothing and exits 0, the same as any clean run.

### 2. Pin what matters

Pinning turns a fact tiger already computes into something enforced on every
later check. While tiger checks a function, it also works out a set of facts
about it, and any of them can be read back on request.

Two of those facts matter for pinning. One is the function's effects, the
full set of things it does anywhere beneath it: allocate, do IO, block,
panic, read time or randomness, mutate, spawn. The other is what the
function writes through its parameters, which tiger calls the function's
frame. Neither one is a finding, so neither fails the build by itself. Both
print only when asked, with `--show-facts`.

```
$ tiger check --show-facts ./...
header.go:34:1: TS-F01: EncodeHeader's effects are alloc — nothing to fix; to make tiger fail the build if they change, add //tiger:effects alloc
```

The fact prints as the exact comment that would sit above the function in the
source. Freezing it into that comment is called pinning. Typing it out by
hand is unnecessary, because `tiger pin <function>` writes the comment
instead.

```
$ tiger pin EncodeHeader
header.go:35: //tiger:effects alloc
header.go:36: //tiger:frame none
```

A pin does nothing to how the function behaves at run time. It changes what
CI enforces from that point forward: the comment becomes a promise, and
every check holds the code to it.

Drain, from the last section, was pinned with no `block` among its effects. A
later change, made by a person or an agent, adds a channel receive inside it,
and the next check fails with this:

```
drain.go:5:1: TS-F01: this function blocks (channel receive at drain.go:8:2) but its //tiger:effects comment doesn't list block — add block to the comment, or remove that code
```

Two things pass that check afterward: reverting the code, or updating the pin
to include `block`. Either way the edit to the pin appears in the diff, so
whoever reviews the change is the one who rules on the new promise. That is
what keeps a pin from drifting quietly out of date: a pin that no longer
matches the code fails the check, so it cannot sit wrong in a comment
unnoticed.

The table below lays out all four cases.

| Condition                               | What tiger does                          | Merge                   |
| --------------------------------------- | ---------------------------------------- | ----------------------- |
| No pin; the function's behavior changes | Nothing (the fact just has a new value)  | Continues               |
| Pin; pin and code agree                 | Silence                                  | Continues               |
| Pin; pin and code disagree              | Blocking finding with the reason         | **Stops**               |
| You edit a pin                          | Nothing; the edit is visible in the diff | Continues, after review |

## Key Concepts in Tiger

### What a surface is

A surface, in tiger's vocabulary, is any point where your code meets something it
does not control. Most codebases have two: an inbound surface and an
outbound surface.

**Inbound Surface**
The inbound surface is the exported functions and methods through which callers
you do not control reach your code. It is also where a function gets pinned,
freezing what tiger has computed about it.

**Outbound Surface**
The outbound surface is where your code reaches the world it does not control: the
disk, the clock, the network, or anything reached through an interface the code
was handed.

The example below shows one of each surface; it is a fragment, not a full program.

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

Tiger checks both surfaces through one fact, the effect set. On the inbound
surface, tiger computes every exported function's effect set, the list of what the
function does besides return its result. Once that function is pinned, tiger
holds it to that list.

The effect set is built from the outbound surface. The clock, the disk, and the
network are the concrete things a function can reach; a direct reach to any of
them shows up in its effect set. Whatever it reaches only through an interface it
was handed does not.

That is why a pin on an exported function is a promise about what that function
reaches on the outbound surface, however many helpers sit between the two. A pin
that lists no clock, no randomness, and no IO is proof that all of those arrive by
injection.

### Nondeterminism is injectable, and a pin proves it

A function is deterministic when the same inputs always produce the same result.
Nondeterminism is the opposite: the same call can return a different result from one
run to the next, because it read something outside its inputs. The things a function
reads that way are the clock, a random number, a fresh ID, and the network. A run that
reads one of those cannot be repeated exactly, so a failure it produced cannot be
reproduced either.

Our rule for this is injection: a function that needs the clock, randomness, or IO
takes it as an interface parameter, so a caller can hand in the real thing in
production and something else in a test. We do not require the something else.
Whether a given test substitutes a dependency or uses the real one is a judgment the
coder makes per test, and for a crypto library the real one is usually the better
choice. What we require is that the choice exists.

Tiger checks that through the effect set. A call made through an interface adds
nothing to a function's effects, because tiger cannot see which implementation will
answer it. A direct call does add an effect: `time` for the clock, `rand` for
randomness, `io` for the disk or the network. So a pinned function whose effects list
none of those is a function whose nondeterminism all arrives by injection, and a
commit that adds a direct read anywhere beneath it fails at the pin.

`Expire` reads the clock through the interface it was given. `ExpireDirect` reads it
directly. Both are pinned as having no effects:

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

Running tiger on that package prints this:

```
$ tiger check ./inject/...
inject/inject.go:22:1: TS-F02: this function reads the clock (time.Now at inject.go:24:17) but its //tiger:effects comment doesn't list time — add time to the comment, or remove the call
tiger: 1 blocking
```

`Expire` passes. `ExpireDirect` fails, and the finding names the call.

Map iteration order is the one source of nondeterminism Go supplies without being
asked, and there is no interface to inject it through. Tiger instead stops it from
reaching any output that is serialized, logged, or hashed.

What this buys is code that can be tested deterministically, a smaller promise than
code that is deterministic. When a test hands in a clock it controls and a store it
controls, everything behind them is a function of its inputs and of what those two
return, so the same test produces the same run and a failure it finds can be
reproduced. When a test hands in the real disk, the run is as repeatable as the disk
is, and that was the coder's decision to make.

Where a test double, a stand-in used only in tests, is worth writing, one that
produces a timeout or a torn write on demand turns those failures into ordinary test
cases, provided the interface was designed with those faults in mind.

Power of Ten has no determinism rule. This discipline is TigerBeetle's: TigerBeetle
found the bugs that made it worth imitating by running its whole system inside a
deterministic simulator, and that only works if every source of nondeterminism sits
behind an interface the simulator controls. Our contribution is to make the
injectable part of that discipline checkable rather than cultural. A direct clock read
beneath a pinned function is not a review comment. It is a blocking finding that names
the call.

### Invariants have names

An invariant is a property of the data that must hold every time the code reaches a
particular point: a header that is exactly twelve bytes long, a checksum that matches the
payload it covers. We require each invariant to be declared once, by name, in a package of
its own, so the analyzer can count where it is checked. In the ledger example that package
is called `inv`, and it declares two:

```go
type ID string

const (
	HeaderChecksum ID = "header-checksum"
	HeaderSize     ID = "header-size"
)
```

The checking itself is done by assertions, which are not the same thing as errors. An
error is an operating condition a caller has to handle, such as a file that does not
exist, a condition the code is right to expect. An assertion failure means the code itself
is wrong, and the only safe response to code that is wrong is to stop before it corrupts
anything.

Go has no assert statement, so we ship a small assert package with no
dependencies, meant to be copied into the project. We keep its assertions enabled
in production, because an assertion that only runs under test guards nothing
else.

Each place the property must hold passes the invariant's constant to
`assert.Invariant` along with the condition to check. In the ledger,
`EncodeHeader` asserts both invariants on the buffer it produces. `DecodeHeader`
asserts the same two on the buffer it is given, so the property is checked
before the bytes go out and again when they come back. If one of the two checks
is written wrong, the other still stops a bad header.

At run time the constant does nothing more than name the invariant in the
failure message. Its real work happens before run time: the analyzer can find every
reference to a named constant, and it cannot find a string inside an
assertion message. That is what lets it enforce two rules on every
declared invariant.

The first rule is that production code must assert every declared invariant
somewhere. A declaration that nothing asserts describes a guarantee the code
does not actually enforce. One assertion site is enough to satisfy the rule, and
only sites outside test files count toward it. A declaration with none fails the
build, and the finding names the invariant and the call to add.

The second rule is that every declared invariant needs a test that violates it,
written with `assert.Violates`, a helper that runs a function and fails unless
that function trips exactly the named invariant:

```go
func TestShortBufferViolatesSize(t *testing.T) {
	assert.Violates(inv.HeaderSize, func() {
		ledger.DecodeHeader(make([]byte, ledger.HeaderSizeBytes-1))
	})
}
```

Deleting that test from the ledger produces this:

```
$ tiger check ./examples/ledger/...
examples/ledger/inv/inv.go:13:2: TS-A09: no test violates invariant inv.HeaderSize — add a _test.go function that calls assert.Violates(inv.HeaderSize, func() { ... })
tiger: 1 blocking
```

We require the violating test because an invariant no test can violate is one of
two things: a property the code can never actually reach, or an assertion
written wrong. Either way, the project wants to know which.

Both rules need to see the constants in `inv` and the assertions and tests in
the packages that import it at the same time, so only the `tiger` command
reports them. The golangci-lint plugin does not.

Naming the invariants costs a handful of constants, and it changes three
things. The constants in `inv` become an accurate list of what the system
guarantees, and they stay accurate because deleting the last assertion of any
invariant fails the build. A search for one constant turns up every site that
defends it. An invariant added only to look thorough costs a declaration, an
assertion site, and a violating test, which is more work than finding a real
one.

### Effects are the fourth oracle

An effect set is tiger's list of the kinds of things a function does besides compute
its result. It exists so a claim like "this function never touches the disk" can be
checked by a machine instead of trusted from a comment.

Tiger builds this list by reading a function's body together with every function it
calls, and each entry comes from a fixed vocabulary: allocating memory, doing IO,
blocking, panicking, reading the clock, reading randomness, mutating a named value,
spawning a goroutine. IO carries a qualifier naming which kind, so a disk write shows
up as `io(disk)` and a network call as `io(net)`. A project can add qualifiers of its own
by attaching one to a surface, so every function that reaches the store carries
`io(database)`.

Nobody writes any of this down, because tiger derives the set for every function from
day one, and `--show-facts` prints it.

Go code already has three checks that each settle one yes-or-no question about a
program, and a check of that kind is called an oracle. The compiler is the first: it
settles whether the program is well formed and well typed, before the program runs,
for every input the program could ever receive. A test is the second: it settles
whether the program does the right thing on an input the author wrote down. It runs
only when the suite runs, so it covers exactly the cases someone thought of and no
others. An assertion is the third: it settles whether some property holds at one
point in the code, at run time, for whatever input actually arrived, in production
as much as under test. That is why we ship an assert package and write our assertion rules
against it.

None of the three can say what a function is allowed to do. The compiler sees no
difference between a function that reads a file and one that adds two integers, as
long as both type-check. Neither a test nor an assertion can see a code path that no
input has taken yet. An effect set is the fourth oracle: it settles exactly that
question, and it settles it the compiler's way, before anything runs, for every
input.

The pin is what turns the effect set into a check. A function with `//tiger:effects`
written above it has its computed set compared against that comment on every run.
When a later change then adds a call that opens a socket, the check fails on that
commit, at the pinned function, without any test having to reach the new call.

A function with no pin still has its effect set computed, but nothing is compared
and nothing prints unless the facts are asked for. An unpinned function has made no
claim about purity or anything else. Purity is a positive claim, and it is written
`//tiger:effects none`.

The effect oracle does not judge whether the effects are appropriate. Tiger can
prove that a function blocks. We give it no opinion on whether a function in that
position ought to block, touch the disk, or spawn anything. That judgment belongs to
whoever reviews the diff, and a pin puts it in one visible place, on the line where
the effect set is written down.

### One pin holds the whole subtree

A pin on a function is a promise about everything that function calls, all the way down
through the code it reaches. That reach is what lets a few pins on a package's exported
functions hold the whole package to the same promise. It follows from how tiger computes
an effect set: tiger walks the function's body and every function reachable from it, and
the set it reports is everything it finds all the way down. So a helper several calls
beneath a function that reads the clock puts `time` in the set of every function above
it. A pin freezes that whole-tree summary, so `//tiger:effects none` on an exported
function says that nothing beneath it allocates, blocks, or touches the outside world
either.

A change anywhere under a pinned function can therefore fail the check even when the
pinned function's own lines are untouched. Here, `Serve` is pinned as pure and calls a private
helper; a later change, made by a person or an agent, makes that helper open a network
connection.

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

The finding is reported at the pinned function rather than at the line that changed. The change
was in `dial`, but `dial` made no promise. `Serve` did, and what tiger checks is whether
`Serve` still keeps it. The call the finding names is always one in `Serve`'s own body,
here `dial`. If `Serve` called a helper named `prepare` and `prepare` called `dial`, the
finding would name `prepare` instead. `prepare` is the call in `Serve`'s body
that the effect comes through, and the path to the cause continues from there.

The fix is one of the two the finding offers: remove the network call, or change the pin
to `//tiger:effects io(net)`. The pin edit is what the reviewer rules on. The check is
exact in both directions. If a later change removes the network call, an `io(net)` pin
fails the same way the `none` pin did. The code no longer does what the pin says.
So a pin cannot describe more than the code does without the check failing. It never
drifts into a superset that means nothing.

We attach pins to exported functions only, because private helpers are the code that
gets split, merged, and renamed during a refactor. A pin on each would have to be
edited along with every reshaping. Nothing is lost by leaving helpers unpinned, because
every helper is already inside the subtree of the exported function that calls it.

The subtree does not stop at the package boundary. A pinned function that calls into
another package is held to whatever that package's code does, and the finding names the
imported function the same way it names a private helper. Where the callee is itself
pinned, tiger reads the callee's pin instead of walking its body again. The
callee's own check already proves that pin's accuracy. Checking stays modular where pins
are dense.

### A frame is where a function writes

A function's frame is the set of locations it writes through its receiver and its
parameters, counting the writes made by anything it calls. Tiger computes the frame
for every function, alongside the effect set, without needing any annotation. The
frame exists so that a bad value can be traced to the functions that could have
written it.

The frame and the effect set answer different questions. The effect set says what
kind of thing a function does, such as allocate memory or read the clock, while the
frame says which state it changes.

When a field holds a value it should not, some function put it there. The frame
narrows the search to whichever functions have that field in their frame. A function
whose frame leaves the field out could not have written it, no matter how much of the
program it touches. Nobody reads the program to find the writer, because the
computed frames already say who is a candidate.

A frame pin is held exact in both directions, the same as an effects pin. Here
`Apply` is pinned to `r.log` and calls a helper that also advances the checkpoint.

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

The finding is reported at `Apply`, the pinned function, and the position it names is
the call to `advance` inside `Apply`'s own body. That is the same convention the
effects check uses.

The check fails the same way in the other direction. Listing `r.pending`, a field
`Apply` never touches, produces a finding that reads "this function never writes
r.pending, yet its //tiger:frame comment lists it", and the finding prints the
corrected comment.

As with effects, a pin goes only on an exported function or method. An exported
function with no pin still has its frame computed, and `--show-facts` prints it as
the comment that would freeze it. `tiger pin` writes that comment. That
is why the `EncodeHeader` run earlier on this page produced a `//tiger:frame none`
line next to the effects line.

Tiger follows a write back to a receiver or parameter through field selections,
element addresses, and pointer dereferences, and no further. A helper that hands back
a pointer into the receiver, or code that copies part of it into a package variable,
breaks that trail, so a write made through either one stays outside the computed
frame. Tiger does not report what it cannot trace, so no finding says so. A frame pin
proves what tiger can trace, and the rule reference records this as a known miss.

### Whole bug classes are excluded, not caught

The benefits list earlier on the page promised that four kinds of bug are excluded rather
than caught, and the difference is where the check is applied: to the shape of the code
that could produce the bug. A linter looks at code for the pattern of a known bug, so it
finds the instances that match its pattern and misses the rest. We instead require a
particular shape of every loop, every switch over a fixed set of values, every `go`
statement, and every blocking `select`, and the shapes we allow are ones in which the bug
cannot occur. Tiger never asks whether a given loop happens to hang. It asks whether the
loop is in an allowed shape, and that question has an answer for every loop.

Four small files, each written the way a Go programmer would write it without
thinking, produce one finding apiece:

```
$ tiger check ./bugs/...
bugs/drain.go:5:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
bugs/status.go:11:2: TS-S08: this switch over Status has no default arm — add default: assert.Unreachable("Status: unhandled value") so a new Status constant fails loudly
bugs/wait.go:4:2: TS-C05: this select blocks with no case that ends the wait on shutdown — add case <-ctx.Done(): return ctx.Err()
bugs/worker.go:4:2: TS-C02: this go statement starts a goroutine nobody owns: nothing says when it exits or ties it to a context — start it through errgroup.Group.Go
tiger: 4 blocking
```

The allowed shape for a loop starts with a stated bound, a condition built from a
constant, a length, or a counter. The `Drain` transcript from earlier on the page, the
loop that ranges over a channel, fails on that requirement alone. A stated bound is
only a promise that the loop could end: a loop can test `len(pending) > 0` in its
condition and never shorten `pending` anywhere in its body. So tiger also looks for
something in the loop that shrinks on every pass and has a floor it cannot cross, the
standard argument that a loop terminates, and we call that measure the loop's variant.
Tiger finds a variant unaided for nearly every real loop. When it does, the loop is
proven and nothing prints. The expression it found is available under `--show-facts`
as the `//tiger:variant` comment that would freeze it. The first finding in the
transcript above is a loop whose shortening happens only inside an `if`, so nothing
shrinks on every pass. The finding names the two ways out: a cap that fails when
it is hit, or a `//tiger:variant` pin naming what shrinks.

We require every switch over a type with a fixed set of values to handle each value
and end in a `default` arm that calls `assert.Unreachable`. The compiler cannot check
that such a switch is complete, because to it an integer constant is only a number. A
value that arrived off the wire and was never in the set must land somewhere that
crashes rather than fall out the bottom of the switch. Once every switch has that arm,
adding a constant breaks each switch that has not considered it, which is the outcome
we want. A type whose set of values is meant to grow is marked `//tiger:openenum`, and
a switch over it keeps a `default` arm as a legitimate catch-all rather than a crash.
In our trials this rule found three real bugs, two of them storage backends dropping
an action in silence.

We require every goroutine to start through a supervisor such as `errgroup.Group.Go`,
which ties it to a context and to code that waits for it to finish. A bare `go`
statement in the middle of a function is a finding, because nothing in it says when
the goroutine exits or what stops it. Starting a goroutine also puts `spawn` in the
function's effect set, so a function several calls up whose pin does not include
`spawn` fails the check on the commit that added the goroutine, however deep it was
added. Our trials found a daemon whose `sync.WaitGroup` was never waited on, so its
`Shutdown` method could return while its HTTP goroutines were still running.

We require every blocking `select` to have a case that ends the wait on shutdown,
either `ctx.Done()` or a channel that is recognizably a shutdown signal. A loop that
runs forever must have the same case in its `select`, so the loop can end. Without
that case a shutdown request has no effect on the wait, so the process stays up until
something kills it. A process that dies partway through a write loses the write. Our
trials found a queue whose Produce, Complete, and Retry waits could not be cancelled,
while its documentation said they could.

## The concepts, in order

### Rules and the two severities

Every rule tiger enforces carries a code and exactly one of two severities. A blocking
finding fails the run. An advisory finding is counted. It prints only when a package
carries more of them than its budget allows, and the next section says what a budget is.

There is no warning tier, on purpose. A rule either blocks or it is not a rule, so
nothing tiger prints can be weighed and set aside.

Advisory severity exists for exactly two things every real codebase carries: an escape
directive, a comment that loosens one rule at one site, and a skipped test. Both are
legitimate and both are worth bounding, so tiger counts them against a per-package budget
instead of pretending they are errors.

### The budget and the ratchet

The budget lives in `tiger.budget.yaml`, a file checked into the repository that
records one number per package per advisory rule: the count of findings that package
is allowed to carry. Under budget, nothing prints. Over budget, the run fails and the
counted findings print.

`tiger budget --write` records the current counts. It can only lower a number. Raising
one is a hand edit, and that edit lands in the diff where the reviewer sees it.

We call this the ratchet: the budget can tighten on its own but only a reviewed commit
can loosen it.

### Directives

A directive is a `//tiger:` comment written on the line above whatever it describes.
Directives are how a person tells tiger something it cannot compute, or freezes
something it did. Its vocabulary is closed, so a typo inside one is a blocking finding
and a directive cannot silently do nothing.

There are three kinds:

1. Pins freeze a computed fact into a contract: `//tiger:effects`, `//tiger:frame`,
   `//tiger:variant`, and the pair `//tiger:requires` and `//tiger:ensures`, which pin
   a precondition on a function's arguments and a postcondition on its result. All of
   them are optional, and `tiger pin` writes the first two for you.
2. Intent declarations state something no analyzer can compute, and the code is held
   to that statement from then on. `//tiger:restrict` says a package forgoes some
   language feature, and `//tiger:openenum` says a type's set of values is expected to
   grow.
3. Escape directives loosen one rule at one site, and each one carries a mandatory
   reason. There is exactly one, `//tiger:batched`, and every use of it counts against
   the budget on every run.

The rules allow exactly one escape directive, because an agent will use any comment
that gets it past a failing check. We admit an escape only when reality, not
convenience, demands it. Some external systems accept one item at a time, and no
rewrite of your code changes that fact. That is what `//tiger:batched <reason>` exists
to name. A general "ignore this rule here" directive does not exist and will not. The
only paths past a finding are a rewrite that complies with the rule, or an escape that
is counted and put in front of a reviewer.

## How review divides

Once code carries pins, restrictions, and invariant declarations, review of it splits
into two jobs. Each gets different treatment.

**Machines check the correspondence.** Whether the code satisfies the invariant it
names, stays inside its pinned frame, terminates, covers every case, matches its
declared effect set, and respects its package restrictions is a mechanical question.
It is answered on every commit. Which checks apply to a function follows from its
computed effect set, wherever in the tree that function happens to sit.

**Humans and AI review the declarations.** Whether this is the invariant the
protocol needs, whether this function should be permitted to touch the disk at all,
whether this limit is true of the hardware it deploys on. No analyzer can answer
those questions, and they are the only ones left.

The declarations form a layer that cuts across every file in the tree. No boundary
sets off a "reviewed part" from the rest, so there is nothing like that to erode.
They are also a small fraction of the lines in that tree, and they change far less
often than the code around them. That is why review attention concentrates on the
place where being wrong costs the most.

A change that touches no declaration and passes CI is a candidate to merge without a
human reading it. A change that does touch a declaration goes to someone who knows
the domain. Nothing else about it needs discussing, because the mechanical
questions have already been answered.

## What tiger will not do

Tiger has no warning tier, no suppression comment scoped to a single call site, and no
informational output on a normal run. When a rule's findings turned out to be noise on
real code during the trials, we removed the rule from tiger entirely rather than demote
it to a warning. Three naming rules went that way, because a smaller set of rules people
actually fix beats a large set people learn to scroll past.

The line tiger holds is the one it can check. Tiger can prove that the code satisfies
the declarations attached to it. Nothing can prove that a declaration matches the world,
and that gap is why the declarations remain the part humans still review.

## Try it

The fastest way to form an opinion about tiger is to point it at code you know well.

```
$ go install github.com/kapetan-io/tiger/cmd/tiger@latest
$ tiger check ./...
```

Expect findings. Our first run on the 42,000-line queue we knew and trusted produced
1,023. A wall of findings is not tiger saying the code is bad. It is the measured distance
between idiomatic Go and the rules.

Read ten of the findings. If most of them name a hazard you would want fixed, gate CI on
`tiger check` and work through them. If they do not, no tool should talk you into it.

To see code that already follows every rule, read [examples/ledger](../examples/ledger) in
the repository. When a finding names a rule you want the reasoning for, the [rule
reference](Tiger%20Rule%20Reference.md) has one entry per rule, with a firing example and
the compliant rewrite. For the full normative detail, the
[specification](Tiger%20Specification.md) is the authority the reference is drawn from.

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
