# Tiger, explained

> Tiger defines a restricted, deterministic dialect of Go and checks that your code
> stays inside it. You run one command and get a verdict. Either the code conforms,
> or you get a list of the exact edits that make it conform. No warnings, no judgment
> calls, and no LLM anywhere in the loop. The same code gets the same verdict, every
> run, on every machine.

`tiger check` · binary verdict · change detection

**This page describes tiger as it is meant to be when it is finished.** Most of what it
says is built, and every transcript on the page is real `tiger check` output. The
exception is the set of checks on surfaces, which are specified and not enforced.
Tiger does not yet check that every nondeterministic call goes through a declared
surface, that each surface has a simulated twin passing the same conformance suite as
the real one, that the interface can express every fault the real one produces, or that
adapters contain no logic. Those checks are described as settled in the Benefits bullet
"You test the failures you can't provoke" and under "Where each benefit comes from". The
rule reference lists what tiger enforces today, and the specification marks each rule
that is not yet built.

## Your agent writes faster than you can read

Your coding agents produce diffs faster than you can read them. Tiger holds the code written by AI to a strict set of proven restrictions, restrictions that make the code deterministic and shut off whole classes of bugs and security holes.

The restrictions come from NASA's Power of Ten, the rules JPL uses for flight software, and from TigerBeetle's TigerStyle, which tiger is named for.

Each rule bans a construct that compiles, reads as idiomatic Go, and fails in production under an input or timing no review considered. An AI reviewer passes it for the same reason a human does. For instance,

- A loop with no provable bound is a denial of service waiting to happen. 
- A goroutine with no owner outlives shutdown and leaks. 
- A `time.Now()` buried deep in your business logic reads a different clock on every run, so a failure you saw once can never be reproduced in a test.

Tiger enforces each rule with its own analyzer, so conformance is not a guess an AI reviewer can make. It forces the AI to write deterministic, safe, testable code.

With tiger there is no `//nolint` equivalent to silence a finding, and no warnings to scroll past, a finding either blocks the merge or tiger doesn't report it, which closes the route an agent would otherwise take past the CI gate. The only way through is the compliant rewrite.

Tiger's second job is change detection. While it checks, tiger computes each function's **effects**, the verbs it performs anywhere beneath it (allocate, do IO, block, read the clock, spawn a goroutine). Freeze a fact like that into a comment called a pin, and a change that silently breaks the promise fails the build. An agent can't drift a pinned fact past you.

### Benefits of using Tiger

**Tiger requires deterministic code**

- **Reproducible errors.** Same inputs, same run, so a bug you saw once you can
  reproduce.
- **Flaky tests disappear.** A test that fails on timing has nowhere to get timing
  from.
- **Agents stop burning tokens on reachability.** In deterministic code, whether a
  state can be reached has an answer, so the agent goes to the fix instead of
  reasoning about interleavings.
- **You test the failures you can't provoke.** Every clock, network, disk, and store
  sits behind an interface with a simulated twin that can produce every fault the
  real one can, so a timeout or a torn write is a test case, not an incident.

**Tiger restricts the language**

- **Whole bug classes can't be written.** Unbounded loops, leaked goroutines,
  dropped switch cases, uncancellable waits. Not caught, excluded.

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
up with a warning count nobody reads anymore. It's not a failure of any particular
linter, it's the nature of advisory output.

Tiger's output is never advisory. It checks whether your code is inside the dialect,
and what it flags are the patterns that turn into subtle bugs, the unbounded loop,
the switch that silently drops a case, the wait that can't be cancelled. Every
finding is a verdict. The build fails, and the finding names the edit that fixes it.
If tiger is wrong, that's a bug in tiger; you file it, the fix lands in the analyzer
with a regression test, and nobody writes a suppression comment.

golangci-lint keeps doing what it does today, catching likely bugs and style
issues, and tiger configures it for you (`tiger golangci --init` generates the
config for the rules existing linters already enforce well, so tiger doesn't
reimplement them). Tiger enforces the dialect.

We chose every restriction in the dialect for decidability, not taste. Each one
exists to make some analysis possible. A bounded loop makes termination checkable, a
supervised goroutine makes shutdown traceable, and a package that declares a
restriction like `no-reflect` (no reflection anywhere in the package) hands the
analyzers a smaller world to reason about. No single restriction is worth much on
its own, and completing a chain of them turns an analysis that's intractable on
ordinary Go into a cheap one tiger runs on every commit.

**What you touch.** One command in CI and locally. Four subcommands total (`check`,
`budget`, `golangci`, `pin`). Two committed YAML files. A handful of `//tiger:`
comments in your source.

**Inside (not your concern).** Thirty-plus analyzers walking your code's structure,
computing what each function does, what it writes, and why each loop ends, then
comparing all of it against the rules and your declarations.

## A day in Tiger Go

The whole workflow is four moves. Write code, read the verdict, fix or declare, and
pin what matters.

You write code → `tiger check` → you fix the findings → you pin what matters

### 1. Run the check

Say you write this. It compiles, it's idiomatic Go, and reviewers wave it through
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

That's a real transcript. The finding line names the rule (`TS-S02`), says what the
code actually does, and names two concrete edits that fix it. You never need tiger
vocabulary to act on a finding. When you want the reasoning behind a rule, look its
code up in the [rule reference](Tiger%20Rule%20Reference.md).

A clean run prints nothing and exits 0; silence is the good news.

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
some agent) later makes the pinned function block on a channel. The next check fails.

```
drain.go:5:1: TS-F01: this function blocks (channel receive at drain.go:8:2) but its //tiger:effects comment doesn't list block — add block to the comment, or remove that code
```

There are two honest exits. Change the code back, or change the pin. A pin edit is
visible in the diff, so the reviewer rules on the new promise. A pin can never go
quietly stale, because a wrong pin is a failed check, not a lie in a comment.

| Condition | What tiger does | Merge |
| --- | --- | --- |
| No pin; the function's behavior changes | Nothing (the fact just has a new value) | Continues |
| Pin; pin and code agree | Silence | Continues |
| Pin; pin and code disagree | Blocking finding with the reason | **Stops** |
| You edit a pin | Nothing; the edit is visible in the diff | Continues, after review |

## Where each benefit comes from

### What a surface is

A surface is where your code meets something it doesn't control, and there are two.
Callers you don't control reach your code through its exported functions. That's the
inbound surface, and it's where pins attach. Your code reaches the world it doesn't
control, the disk, the clock, the network, through interfaces it is handed. That's
the outbound surface. Every function sits on one side of one of them.

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

Tiger checks what crosses each edge. On the inbound edge it computes every exported
function's effects and, once you pin them, holds the function to the pin. On the
outbound edge it checks that time, randomness, and IO arrive only through a declared
interface, and that the interface has a simulated twin held to the same tests as the
real one. The adapters that implement the outbound surface are production code and
get a rule of their own. A pin on the inbound edge is therefore a claim that reaches
all the way down to the outbound one.

### Nondeterminism enters only through a surface

TS-T01 says time, randomness, and IDs are injected, and TS-I01 says how. Any function
whose effect set contains `io`, `rand`, or `time` has to obtain it from a surface it
was handed as a parameter, so no function reaches for `time.Now()` on its own. The
one source of nondeterminism Go injects for you, map iteration order, is stopped by
TS-T02 from reaching a serialized, logged, or hashed output. Everything behind the
outbound surface is then a function of its inputs and of whatever the surfaces
returned. Rerun it with the same answers and you get the same run, which is what
makes a failure reproducible, a timing-dependent test impossible to write, and the
question "can this state be reached" one the trace answers instead of the agent.

The adapters that implement the outbound surface are tested as production code.
TS-I03 requires every surface to have a production implementation and a simulated
one, and both must pass one conformance suite. TS-I07 makes fidelity a configuration
rather than a code path, so the same suite runs fully simulated, partially simulated,
and fully real. The fully-real run exercises the adapters that ship, and the
simulated run exercises everything else under faults reality won't produce on demand.
TS-I05 requires the interface to express every fault the real thing can, so a store
that can tear a write has a torn-write case in its interface, and TS-I06 requires
adapters to contain no logic, because a branch in an adapter is a decision the
simulator never sees. Those rules together are why a timeout or a torn write is a
test case. The fault is in the interface, the twin can produce it on demand, and the
suite proves the twin and the real adapter agree everywhere else.

None of this comes from Power of Ten, which has no determinism rule. It is
TigerBeetle's discipline. TigerBeetle found the bugs that made it worth imitating by
running its whole system inside a deterministic simulator, and that only works if
every source of nondeterminism is behind an interface the simulator controls. Tiger's
contribution is to make the discipline checkable rather than cultural. A clock read
outside a surface isn't a review comment. It's a blocking finding that names the
call.

### Invariants have names

An invariant is a property of the data that must hold every time the code reaches a
particular point, such as a header being exactly twelve bytes long or its checksum
matching its payload, and in tiger each one is declared once, by name, so that the
analyzer can count where it is checked. The checking itself is done by assertions. An
assertion is not an error. An error is an operating condition the caller has to handle,
like a file that does not exist, while an assertion failure means the code itself is
wrong, and the only safe response to wrong code is to stop before it corrupts anything.
Go has no assert statement, so tiger ships a small assert package with no dependencies,
meant to be copied into the project, and its assertions stay enabled in production,
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

Two rules follow from that. The first is that production code has to assert every
invariant the project declares, somewhere, because a declaration that nothing asserts
describes a guarantee the code does not enforce. One assertion site is enough, and only
sites outside the test files count. A declaration with none fails the build with a
finding that names the invariant and the call to add. The second is that every declared
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

The reason for the second rule is that an invariant no test can violate is one of two
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
its result, and it exists so that a claim like "this function never touches the disk"
can be checked by a machine instead of trusted from a comment. Tiger builds the list by
reading the function's body together with every function it calls, and the entries come
from a fixed vocabulary: allocating memory, doing IO, blocking, panicking, reading the
clock, reading randomness, mutating a named value, and spawning a goroutine. IO comes
with a qualifier naming which kind, such as `io(disk)` or `io(net)`, and a project can
add qualifiers of its own by attaching one to a surface, so that every function reaching
the store carries `io(database)`. Nobody writes any of this down. The set is derived for
every function from day one, and `--show-facts` prints it.

What makes the set worth computing is the question it can answer. Go code already has
three checks that each settle one yes-or-no question about a program, and a check of
that kind is called an oracle. The compiler is the first: it settles whether the program
is well formed and well typed, and it does so before the program runs, for every input
the program could ever receive.

A test is the second. It settles whether the program does the right thing on an input
the author wrote down, at run time whenever the suite runs, so it covers exactly the
cases someone thought of and no others. An assertion is the third. It settles whether some
property holds at one point in the code, at run time, for whatever input actually
arrived, in production as much as under test, which is why Go's lack of an assert
statement matters enough that tiger ships an assert package and writes its assertion
rules against it. None of the three can say what a function is allowed to do. The
compiler sees no difference between a function that reads a file and one that adds two
integers, as long as both type-check, and neither a test nor an assertion can see a
code path that no input has taken yet.

An effect set is the fourth oracle. It settles exactly that question, and it settles it
the compiler's way rather than the way a test or an assertion does: before anything
runs, for every input. The pin is what makes it a check. A function with
`//tiger:effects` above it has its computed set compared against the comment on every
run, so when a later change adds a call that opens a socket, the check fails on that
commit, at the pinned function, without any test having to reach the new call. A
function with no pin still has its set computed, but nothing is compared and nothing
prints unless the facts are asked for. Unpinned is not a claim of purity or of anything
else. Purity is a positive claim, and it is written `//tiger:effects none`.

One thing the effect oracle leaves alone is whether the effects are appropriate. Tiger
can prove that a function blocks, and it has no opinion on whether a function in that
position ought to block, or touch the disk, or spawn anything. That judgment belongs to
whoever reviews the diff, and a pin puts it in one visible place, on the line where the
effect set is written down.

### One pin holds the whole subtree

A pin on a function is a promise about everything that function calls, not only about
its own body, and that reach is what lets a few pins on a package's exported functions
hold the whole package to a promise. The reach follows from how tiger computes an
effect set. It walks the function's body and every function reachable from it, and the
set it reports is everything it finds all the way down, so a helper several calls
beneath a function that reads the clock puts `time` in the set of every function above
it. A pin freezes that whole-tree summary, so `//tiger:effects none` on an exported
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
to `//tiger:effects io(net)`. A pin edit shows up in the diff, so a reviewer rules on the
new promise.

The check is exact in both directions. If a later change removes the network call, an
`io(net)` pin fails the same way the `none` pin did, because the code no longer does
what the pin says. A pin cannot describe more than the code does without the check
failing, so it never drifts into a superset that means nothing.

This is why pins attach to exported functions only. Private helpers are the code that
gets split, merged, and renamed during a refactor, and a pin on each of them would have
to be edited along with every reshaping. Nothing is lost by leaving them unpinned,
because every helper is already inside the subtree of the exported function that calls
it, which is what the specification means when it says sparse pins give dense
enforcement.

The subtree does not stop at the package boundary. A pinned function that calls into
another package is held to whatever that package's code does, and the finding names the
imported function the same way it names a private helper. Where the callee is itself
pinned, tiger reads the callee's pin instead of walking its body again, because the
callee's own check already proves that pin accurate, so checking stays modular exactly
where pins are dense.

### Whole bug classes are excluded, not caught

The benefits list says four kinds of bug are excluded rather than caught, and the
difference is where the check is applied. A linter looks at code for the pattern of a
known bug, and so it finds the instances that match its pattern and misses the rest.
Tiger instead requires a particular shape of every loop, every switch over a fixed set
of values, every `go` statement, and every blocking `select`, and the shapes it allows
are ones in which the bug cannot occur. The question it asks is never
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

The allowed shape for a loop starts with a stated bound, meaning a condition built
from a constant, a length, or a counter, and the `Drain` transcript earlier in this
document is a loop that fails on that requirement alone. A stated bound is only a
promise that the loop could end, though. A loop can test `len(pending) > 0` and never
shorten `pending`, so tiger goes further and looks for something in the loop that
shrinks on every pass and has a floor it cannot cross, which is the standard argument
that a loop terminates. It finds one unaided for nearly every real loop, and when it
does the loop is proven, nothing prints, and the expression it found is available under
`--show-facts` as a fact ending in the `//tiger:variant` pin that would freeze it. The
first finding above is a loop whose shortening happens only inside an `if`, so nothing
shrinks on every pass, and the finding names the two ways out: a cap that fails when
hit, or a `//tiger:variant` pin naming what shrinks.

Switches are the second shape. A `switch` over a type with a fixed set of values must
handle each value and end in a `default` arm that calls `assert.Unreachable`. The
compiler cannot check that such a switch is complete, because an integer constant is
only a number to it, and a value that arrived off the wire and was never in the set
must land somewhere that crashes rather than fall out the bottom of the switch. Once
every switch has that arm, adding a constant breaks each switch that has not
considered it, which is the outcome the rule exists for. A type whose set of values is
meant to grow is marked `//tiger:openenum`, and a switch over it keeps a `default` arm
as a legitimate catch-all rather than a crash. In the trial runs this rule found three
real bugs, two of them storage backends dropping an action in silence.

For goroutines, the allowed shape is that each one starts through a supervisor, such as
`errgroup.Group.Go`, that ties it to a context and to code that waits for it to finish.
A bare `go` statement in the middle of a function is a finding, because nothing in it
says when the goroutine exits or what stops it. Starting a goroutine also puts `spawn`
in the function's effect set, so a function several calls up whose pin does not include
`spawn` fails the check on the commit that added the goroutine, however deep it was
added. The trials found a daemon whose `sync.WaitGroup` was never waited on, so its
`Shutdown` method could return while its HTTP goroutines were still running.

The last shape is the wait. A `select` that blocks must have a case that ends the wait
on shutdown, either `ctx.Done()` or a channel that is recognisably a shutdown signal,
and a loop that runs forever must have the same case in its `select` so that the loop
can end. Without that case, a shutdown request has no effect on the wait, so the process
stays up until something kills it, and a process that dies partway through a write
loses the write. The trials found a queue whose
Produce, Complete, and Retry waits could not be cancelled, while its documentation
said they could.

## The concepts, in order

### Rules and the two severities

Every rule has a code like `TS-S02` and exactly one of two severities. **Blocking**
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
3. **Escape hatches** (counted). Loosen one rule at one site, with a mandatory
   reason. There is exactly one, `//tiger:batched`, and every use is counted against
   the budget on every run.

Why only one escape hatch? Because an agent will use any comment that gets it past
a failing check. An escape is admitted only when reality, not convenience,
demands it. Some external systems accept one item at a time, and no rewrite of your
code changes that, so `//tiger:batched <reason>` exists. A general "ignore this rule
here" directive doesn't exist and won't. The only paths are the compliant rewrite or
a counted, human-reviewed escape.

## How review divides

Once the declarations exist, review splits into two jobs that get different
treatment.

**Machines check the correspondence.** Does the code satisfy the invariant it names,
stay inside its pinned frame, terminate, cover every case, match its declared effect
set, respect its package restrictions. All of it mechanical, all of it on every
commit.

**Humans and AI review the declarations.** Is this the invariant the protocol
actually needs. Should this function be permitted to touch the disk at all. Is this
limit true of the hardware you deploy on. Questions no analyzer can answer, and the
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

## What adoption looks like

The queue's first trial run produced 1,023 blocking findings, and the git server's
produced 712. That's what pointing a strict dialect at
good, existing, hand-written Go looks like, and it's why adoption is a decision about
a codebase's future, not an afternoon's chore. On a fresh codebase where the code is
AI-written against the rules from day one, the count starts at zero and there's
nothing to retrofit.

1. **Wire up the auto rules** (one command). `tiger golangci --init` generates the
   golangci-lint config for the rules tiger delegates. `tiger golangci` audits an
   existing config against that baseline.
2. **Run the check, fix what it finds** (the real work). `tiger check ./...`. Real
   bugs first (our trials found ten). For loops an external system forces on you,
   declare `//tiger:batched` with the reason.
3. **Record the budgets** (one command). `tiger budget --write` records each
   package's current advisory counts. Expect the first check to fail until this runs
   once; a package with findings and no budget row has a budget of zero.
4. **Pin as you go** (ongoing). `tiger pin` on the functions whose behavior matters
   most. Sparse pins are fine, a pin covers everything the pinned function calls.

The escape hatch earned its keep in our trials. The queue had 24 loops that drain a
database cursor, a shape only the backing store bounds. Each got `//tiger:batched`
with its reason. The loop-bound rule went from 37 findings to 11, every waiver became
a counted, reviewable line in the budget, and the one real deadlock among those
findings kept firing.

> **Plugin caveats.** Tiger also runs inside golangci-lint as a plugin, but two
> things need the real CLI. Budgets, because the plugin has no end-of-run step to
> count against them, so it reports every counted finding as an issue. And the three
> whole-program rules that need evidence from every package at once (is each
> invariant asserted somewhere, does a test violate it, does an interface have a
> second implementation). Run `tiger check` in CI; use the plugin for editor
> integration.

## What tiger will not do

No warnings. No per-site suppression comments. No "informational" output on a normal
run. When a rule's findings turned out to be noise on real code, we removed the rule
from tiger entirely rather than demote it to a warning; three naming rules died
exactly that way during the trials. A smaller set of rules people actually fix beats
a large set people learn to scroll past.

Tiger can prove the code satisfies the declarations; nothing can prove the
declarations match the world. Whether the invariant is the right one, whether the
limit is true of your hardware, whether this function should be allowed to touch the
disk at all, those questions live outside the code. That's exactly why the
declarations are the part humans still review.

## Try it

The fastest way to form an opinion about tiger is to point it at code you know well.

```
$ go install github.com/kapetan-io/tiger/cmd/tiger@latest
$ tiger check ./...
```

Expect findings. Remember our numbers, 1,023 on the 42,000-line queue we knew and
trusted. A wall of findings isn't tiger telling you your code is bad, it's the
measured distance between idiomatic Go and the dialect. Read ten of them. If most
name a hazard you'd want fixed, work the adoption steps above. If they don't, no tool
should talk you into it.

To see code that already lives inside the dialect, read
[examples/ledger](../examples/ledger) in the repository. When a finding names a rule
you want the reasoning for, the [rule reference](Tiger%20Rule%20Reference.md) has one
entry per rule, with a firing example and the compliant rewrite. And when you want
the full normative detail, the [specification](Tiger%20Specification.md) is the
authority the reference is drawn from.

---

The trial numbers come from tiger's runs on two real codebases before the rules
settled. Every transcript on this page is live `tiger check` output. The runs under "A
day in Tiger Go" are against `examples/ledger`. The `Serve` and `dial` run and the
four-file `bugs` run are against a scratch module, since the ledger has no function with
helpers beneath it and no loop, switch, goroutine, or wait written in the shapes tiger
rejects. The `Clock` and `Expire` snippet is illustrative and is not in the ledger.
