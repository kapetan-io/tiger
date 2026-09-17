`tiger check` · binary verdict · change detection

**This page describes tiger as it is meant to be when it is finished.**

## Overview
Tiger is a static analyzer for Go, built for code that AI agents write. It holds that code to the restrictions NASA's [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) and TigerBeetle's [TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) put on flight and database software, so the result is testable and free of whole classes of production bugs. Every loop has a bound, every goroutine has an owner, and every clock, random source, and IO call is passed in, so any function can be tested in isolation. A run either passes or fails. Tiger has no warnings and no suppression comment, so code that fails the check is rewritten until it passes. The analysis is deterministic static analysis and never a language model, so the same code gets the same verdict every time.

In addition to enforcing the coding standard, tiger also computes the **effects** of each function on the public surface. An effect is a side effect of the method, or of any call beneath it (allocate, do IO, block, read the clock, spawn a goroutine). Tiger lets you record a function's effects in a comment called a **pin**. During review, any change that silently breaks the pinned effects fails a tiger check. The goal is that an agent can't change the effects of the code without your knowledge. For example, if your architecture only allows database interaction via the `Store` interface, and the AI writes a function which talks to the database outside of the `Store` interface, the change in effects tells the reviewer that this PR changed more than code. It changed the architecture.

### Benefits of using Tiger Coding Standards
The tiger coding standard forces the code to abstract all inputs and outputs to your software. Gary Bernhardt's [Boundaries](https://www.destroyallsoftware.com/talks/boundaries) talk and Alistair Cockburn's [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture) describe why that boundary matters.

This provides the following benefits.

**Tiger makes nondeterminism injectable**
- **Failures are reproducible.** When a test supplies the clock and the store, the
  same inputs give the same run.
- **Avoids flaky timing tests.** A test that controls the clock has no timing
  to fail on.
- **Agents debug from a reproduction.** Abstracting inputs and outputs makes a production issue easier for an AI agent to reproduce. An agent that can rerun the failure can fix it, instead of reasoning about which timing produced it.

**Tiger restricts the language**
The tiger coding standard follows rules [proven on flight software](https://doi.org/10.1145/2560217.2560218) to eliminate large classes of bugs and security issues. It also keeps complexity down by giving the AI a [cyclomatic complexity](https://doi.org/10.1109/TSE.1976.233837) limit of 10 and a [cognitive complexity](https://www.sonarsource.com/docs/CognitiveComplexity.pdf) limit of 15, so code complexity cannot grow unchecked with each iteration of AI changes.
- **Eliminates whole classes of bugs and security issues.** Unbounded loops, leaked goroutines,
  dropped switch cases, uncancellable waits, all excluded.
- **Simple code is checkable code.** Simplicity enhances verifiability and maintainability.
- **Avoids mistakes.** Tiger bans language features which are frequently misused or that [contribute disproportionately](https://www.youtube.com/watch?v=GRJtYwneG2Q) to errors in code, as [MISRA C](https://misra.org.uk/product-category/misra-c-2/) does for C.

**Tiger lets you declare effects**
Go is an imperative language, so its functions can have side effects, and in most cases this is acceptable and a strength of the language. However, AIs (like humans) can get lazy. Instead of rewriting entire swaths of code to do the right thing, they will often opt to do the wrong thing, bringing unintended effects that contradict good code architecture.

The effects system in tiger is the canary in the coal mine, notifying the reviewer when unexpected side effects are changed or added to the code base.

- **An agent can't change a pinned effect quietly.** Every check recomputes a pinned
  function's effects and compares them to the pin, so a commit that adds IO or a clock
  read anywhere beneath the function is reported by tiger.
- **A pin is never out of date.** A pin that disagrees with the code in either direction
  is a failed check rather than a stale comment, so every pin in a passing tree describes
  the code as it is.
- **Reviews surface effect changes, not diffs.** If an agent adds an effect deep in the tree that the pin above it doesn't declare, the check fails at the top-level pin, and the reviewer sees the change without reading the call tree below it.
- **A few pins can cover the entire source tree.** One pin on an exported function holds every call beneath it, across packages, to the same effect promise.

**Tiger has no warnings**
AIs (like humans) can get lazy, and often just want to complete the task instead of writing quality, maintainable code. To this end, tiger disallows `nolint` and any other way of bypassing a real issue. Holzmann's [tenth rule](https://spinroot.com/gerard/pdf/P10.pdf) is the same: compile with all warnings on and allow none, because [one new warning among thousands is invisible](https://www.youtube.com/watch?v=GRJtYwneG2Q).

- **The agent can't bypass the check.** A run fails or it passes. Tiger has no
  warnings and no suppression comment, so the agent's only path forward is a
  rewrite.
- **Human review should be about behavior and intent, not code.** If a change passes your test suite and no pins were changed, the human reviewing the PR can focus on intent and behavior instead of code.

**Tiger enforces code structure**
Taken directly from TigerStyle and Holzmann's Power of Ten rules for mission-critical code, tiger enforces code structure that is proven to result in high-quality, reliable software.

- **Assertions are dense and stay on in production.** Every function asserts its
  preconditions on entry and its postconditions before return, which by itself puts a
  package past [Holzmann's two assertions per hundred lines](https://spinroot.com/gerard/pdf/P10.pdf). An assertion fails on
  the line where an assumption stops holding, instead of somewhere else ten minutes later.
  A 2006 study of two Microsoft components found that fault density after release fell as
  assertion density rose ([Kudrjavets, Nagappan, and Ball](https://doi.org/10.1109/ISSRE.2006.14)).
- **State has one owner.** A type marked as an owner has no exported fields, no mutex, and
  cannot be copied, and no exported method hands out a pointer, slice, or map into its
  state. Nothing outside the package can reach the state, so the data race is designed out
  rather than defended against.
- **[Data lives at the smallest scope.](https://spinroot.com/gerard/pdf/P10.pdf)** There is no package-level mutable state, and every
  variable is declared where it is first used, so when a value is wrong the functions that
  could have written it are few and tiger can name them.
- **The dependency graph is written down.** Each package declares what it may import and
  what it may use, and the check fails a package that reaches past its declaration.
  Layering is the part of architecture a tool can enforce.
- **Goroutines follow one set of rules.** Every goroutine has an owner and a context that
  ends it. A package shares state through channels or through a mutex, never both, and no
  wait blocks without a way to cancel it.
- **Interfaces say what they mean.** Exported functions take domain types, not bare
  strings and integers, so a value nobody checked cannot reach them. An interface exists
  only where a second implementation uses it, and a surface interface can express every
  fault its specification permits.

## golangci-lint and tiger's own analyzers
A project that already runs golangci-lint is enforcing part of tiger's rule set,
because for those rules an off-the-shelf linter exists. We never reimplement a rule such a
linter enforces. The rules no linter can check are the other part, and for those tiger
carries 33 analyzers of its own, run by `tiger check`. The specification calls them the
custom half, and the linter side the auto half. On that side what tiger adds is the
configuration: it writes the file that turns each linter on at the setting the rule
requires, and it audits that file against the rules. The linter itself keeps running as
before.

Among the rules golangci-lint enforces:

- A function is at most 70 lines (funlen) and a line at most 100 columns (lll).
- A switch over a closed set of values lists every case, and a default arm does not excuse
  a missing one (exhaustive).
- Every returned error is handled (errcheck), and no function uses a naked return
  (nakedret).
- There are no init functions (gochecknoinits) and no package-level mutable state
  (gochecknoglobals).
- A domain package may not import unsafe, reflect, or math/rand (depguard).
- No code calls time.Now, time.Sleep, time.After, or panic directly (forbidigo).

`tiger golangci --init` writes a `.golangci.yml` for a new project from the rule registry,
the table mapping each rule to its linter and required setting, with every linter the auto
rules need enabled and every setting at the rule's value. It refuses to run if a config
already exists. `tiger golangci` with no flag audits an existing config against that same
baseline. A required linter that is not enabled, or a setting that is missing or differs
from the baseline, is a finding that names the rule and the change that restores it. Extra
linters and settings pass. The comparison is exact, so a stricter value fails too.
Generation and audit read the same table, so a generated config passes its own audit. No
rule enters the registry without being both generated and audited.

After the funlen limit in a generated config is changed from 70 to 80, the audit prints
this and exits 1:

```
$ tiger golangci
TS-S04: linters.settings.funlen.lines is 80, but tiger's baseline for the rule "hard limit of 70 lines per function" is 70 — set it to 70 (tiger compares exactly, so a stricter value also fails)
tiger: 1 auto rules unenforced
```

With the value back at 70 it prints nothing and exits 0.

golangci-lint fails its run on any issue, and the generated config removes its caps on how
many issues it reports. Neither half accepts a suppression comment. Tiger flags any
`//nolint` as a finding, and `tiger check` has no suppression comment of its own. Its one
escape directive, `//tiger:batched`, is counted against a per-package
budget on every run. CI runs golangci-lint with the generated config and `tiger check
./...`, and both must pass.

The custom analyzers check what a function does, anywhere beneath it: its effects, what it
writes (its frame), whether every loop has a bound, whether every goroutine has an owner,
and whether the code matches its pins. A few rules are
whole-program, decided once every package has been visited, and only `tiger check` reports
those. The same analyzers also run as a golangci-lint plugin, minus the whole-program
rules and the budget.

### What counts as a rule

A rule in tiger is a question a tool can answer from the code. "Does this loop have a
bound" is a rule, because an analyzer can check it. "Does this look risky" is not, because that is a subjective statement. Tiger's analysis is deterministic static analysis and never a language model, so the same code gets the same verdict on every run.

The inspiration for tiger's static checking comes from Gerard Holzmann's work on flight software at NASA's Jet Propulsion Laboratory. He checked the code of past missions against the rules their own developers said they supported and found the supported rules were not followed, because none of them had ever been checked by a tool. A rule nobody checks is not followed, even by people who agree with it, or in [Holzmann's words](https://www.youtube.com/watch?v=GRJtYwneG2Q), "If your rule is not checkable, don't put it in the standard." He wrote the ten rules that could be checked, built the [Cobra](https://spinroot.com/cobra/) analyzer to check them, and the rules became the [coding standard](https://everyspec.com/NASA/NASA-JPL/JPL-D-60411_VER-1_32832/) for all software written at JPL. Tiger follows that. It has no opinion about whether a change is correct, only about whether the change follows the coding standard tiger defines.

## Using Tiger

### 1. Run the check

`tiger check` reads the packages of a module and prints one line for each finding. A
developer runs it as `tiger check ./...` from the command line, as one step of four: write
the code, run the check, fix what it finds, then pin what matters. A run that finds
nothing prints nothing and exits 0. A run that finds something prints a line per finding
and a count of blocking findings, the kind that fails the run, then exits 1.

The function below compiles, is idiomatic Go, and reviewers approve code like it every
day.

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

`tiger check ./...` on its package prints one finding.

```
$ tiger check ./...
drain.go:6:2: TS-S02: this loop ranges over a channel, so it ends only when some other goroutine closes the channel — add a counter cap that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
tiger: 1 blocking
```

A finding line names the rule, states what the code actually does, and names two ways to
fix it. Drain's loop ranges over a channel, so when it ends depends on some other
goroutine closing that channel. The loop itself has no way to bound that ending. The first
fix is a counter that fails once it hits a cap. The second is an event loop that selects
on the context, so a cancellation ends it. Either one bounds the loop.

### 2. Pin what matters

A pin freezes a fact tiger already computes, so every later check enforces it. While
`tiger check` runs, it also computes a fact for every function: the current value of
something a pin can freeze. That something might be an effect set, what the function does;
a frame, what it writes; or a loop variant, the expression that shows a loop ends. A fact
carries no severity and is never counted. It prints only under the `--show-facts` flag on
`tiger check`. The output is in pin syntax. Because facts never add to the blocking count,
a run that prints only facts still exits 0. This transcript is `tiger check --show-facts
./...`, run from inside `examples/ledger/`:

```
$ tiger check --show-facts ./...
header.go:34:1: TS-F01: EncodeHeader's effects are alloc — nothing to fix; to make tiger fail the build if they change, add //tiger:effects alloc
```

One fact this prints is the function's effect set: what it does, over a closed vocabulary
of `alloc` (allocates), `io` (does IO, over disk, network, exec, or the environment),
`block`, `panic`, `rand` and `time` (reads randomness or the clock), `mutate(x)` (writes
to a named location), and `spawn`. The set is transitive by construction, so it includes
everything reachable in the function's whole call chain. When nothing in that chain does
any of them, the set is empty. That is purity, spelled `none` in pin syntax.

Another fact is the function's frame: the set of locations reachable from its parameters
and receiver that it writes.

A fact prints as the exact comment a developer would paste above the function, because
fact output and pin syntax are one format. `tiger pin <Name>` resolves the named function,
takes the facts the same analyzer run reports for it, and writes them as `//tiger:`
comments at its declaration, insert-only:

```
$ tiger pin EncodeHeader
header.go:35: //tiger:effects alloc
header.go:36: //tiger:frame none
```

A pin is a Go comment, and it does not change how the program runs. `tiger check` enforces
it on every later run, including in CI. A version of `Drain` with no channel in it is
pinned `//tiger:effects none`, so `block` is not among its listed effects. A later change
adds a channel receive to its body, an operation that blocks. Checking it now fails:

```
$ tiger check ./...
drain.go:5:1: TS-F01: this function blocks (channel receive at drain.go:8:2) but its //tiger:effects comment doesn't list block — add block to the comment, or remove that code
drain.go:8:2: TS-C05: this channel operation blocks outside a select, so shutdown can't interrupt it — wrap it in a select that also has case <-ctx.Done(): return ctx.Err()
tiger: 2 blocking
```

The second line in that transcript is a separate rule about a channel receive outside a
select.

`tiger pin` refuses to overwrite an existing pin that disagrees with the computed fact.
It prints the disagreement and exits 1: the fix is to change the code or edit the pin by
hand.

There are two ways to pass the check: change the code back, or change the pin. A pin edit
is visible in the diff, so the reviewer rules on the new promise. A pin cannot go stale: a
wrong pin is a failed check, not a lie in a comment.

| Condition                               | What tiger does                          | Merge                   |
| --------------------------------------- | ----------------------------------------- | ----------------------- |
| No pin; the function's behavior changes | Nothing (the fact just has a new value)  | Continues               |
| Pin; pin and code agree                 | Silence                                  | Continues               |
| Pin; pin and code disagree              | Blocking finding with the reason         | **Stops**               |
| You edit a pin                          | Nothing; the edit is visible in the diff | Continues, after review |

## Key Concepts in Tiger

### What a surface is

A surface is where the code meets something it doesn't control. Tiger's model has two of
them, one for what calls into the code and one for what the code calls out to.

**Inbound Surface**

The inbound surface is the exported functions and methods a caller reaches. Pins attach
there and nowhere else.

**Outbound Surface**

The outbound surface is the set of interfaces the domain calls to reach the world: the
store, the clock, the network.

Both appear in this snippet:

```go
// Clock is an outbound surface. Production hands in the wall clock and a
// test hands in one it controls.
type Clock interface {
    Now() time.Time
}

// Prune is on the inbound surface. It reads time only through the clock
// it was given.
func Prune(clock Clock, entries []Entry) []Entry {
```

Tiger checks both surfaces through the effect set it computes for every function. The
pinned function is the one on the inbound surface, and what the pin lists is
what it reaches on the outbound surface. A call made directly, to the standard library or
to another function tiger can statically resolve, contributes its effect to the caller's
computed set. A call made through an interface value contributes nothing, because the
analyzer cannot statically know which concrete implementation will run. A pinned
function whose set lists none of `time`, `rand`, or `io` therefore receives every
nondeterministic input by injection, through an interface, rather than by a direct call.

A pin on an exported function is a promise about what the function reaches on the
outbound surface through every helper beneath it.

### Nondeterminism is injectable, and a pin proves it

A function is deterministic when the same inputs always produce the same result.
Nondeterminism is the reverse: the same call can return a different result from one run
to the next. That happens when the call reads one of the inputs that make a run
unrepeatable: time, randomness, IO, scheduling, or identity. A run that reads one
directly cannot be handed a substitute, so no test can control it. An uncontrolled run
cannot be replayed exactly on failure.

Our rule is that every nondeterministic input is injectable. A function that needs the
clock, randomness, or IO takes it through an interface, such as `Storage`, so a caller
hands in the real thing in production and something else in a test. The IO stays swappable
this way, but it is still performed. Whether that something else is a substitute or the
real dependency is the coder's call, made per test. Tiger requires no fake, mock, or
simulation to exist, and for a crypto library the real one is usually the better choice.
Where a coder writes a substitute for fault-injection testing, review judges which faults
it injects and whether the interface can express them, such as a torn write, a misdirected
write, or an fsync that lies. No analyzer checks either judgment.

Tiger checks that rule through the effect set, the same computed fact a pin freezes. The
effects analyzer resolves direct calls, method calls on concrete types, and function
literals with a known target, and anything else is the known-miss the surface section
described. A direct call adds an effect: `time` for the clock, `rand` for randomness, or
`io` with a qualifier such as `disk` or `net`. A pinned function whose effects list none
of them gets all its nondeterminism by injection. A pin bounds a function's entire
subtree, so a commit that adds a direct read anywhere beneath it fails at the pin. The
finding names the introducing call.

`Expire` reads the clock through the interface it was given; `ExpireDirect` reads it
directly. Both are pinned as having no effects.

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

`Expire` passes with no finding. `ExpireDirect` fails, and the finding names the call it
made.

Map iteration order is a separate source of nondeterminism. Go deliberately randomizes
the order it visits a map's entries. That order is not a value passed into a function, so
there is no interface to inject and no direct call for the effects analyzer to see. Tiger
checks it with the `maporder` analyzer, banning every map range except where the loop body
matches one of a fixed set of shapes known not to depend on order. The finding suggests
ranging over sorted keys instead.
The check is a heuristic rather than a proof, so the spec adds a rule that runs the test
suite twice and diffs the output.

Once a test supplies a clock and a store it controls, every result downstream is a
function of the test's inputs and the clock's and store's answers. That is what injection
provides. Handing in the real clock or disk instead makes the run only as repeatable as
that dependency.

Running a whole system inside a deterministic simulator found the bugs that made
TigerBeetle worth imitating. Their simulator, the
[VOPR](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/internals/vopr.md), runs the
real consensus and storage code in one process with simulated time, network, and disk faults,
a technique [FoundationDB](https://doi.org/10.1145/3448016.3457559) described in
[2014](https://www.youtube.com/watch?v=4fFDFbi3toc). Enabling simulation testing is why we require core logic to
be deterministic with time, randomness, and IDs injected. Tiger enforces the injectable
half of that discipline through the effects analyzer at the pin, where a finding is
blocking and fails the run. It leaves the fault-injection half to review. This discipline
is TigerBeetle's. NASA's Power of Ten has no determinism rule.

### Invariants have names

An invariant is a property of the data that must hold every time the code reaches a
particular point: a header exactly twelve bytes long, or a header whose checksum matches
its payload. We require each invariant to be declared once, by name, so an analyzer can
find every place it is checked, instead of grepping for a string buried in an assertion
call. Declaring it this way turns a property no analyzer could reliably grep for into
reference counting and set equality.

Checking is done by assertions rather than errors: an error is an expected condition the
caller must handle, such as a missing file. An assertion failure means the code itself is
wrong, and the [only correct response is to
crash](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) before it
corrupts anything. That is why assertions stay enabled in production, since an assertion
that runs only under test guards nothing. The crash turns a correctness bug into a liveness
bug, which is the better of the two. [Curiosity landed on
Mars](https://www.youtube.com/watch?v=GRJtYwneG2Q) with its assertions enabled, because an
assertion that fails during a landing at least says in telemetry what went wrong. Since Go
has no assert statement, we ship a small assert package with no dependencies beyond the
standard library.

An invariant is declared as a package-level constant of a named string type, such as `type
ID string`. Tiger recognizes it by use, as the ID argument to some `assert.Invariant` or
`assert.Violates` call somewhere in the module, whatever package declares it. The ledger
groups them this way, in a package named `inv`:

```go
type ID string

const (
	HeaderChecksum ID = "header-checksum"
	HeaderSize     ID = "header-size"
)
```

Each place the property must hold passes the constant to `assert.Invariant(id, cond)`,
which panics when `cond` is false with the message `assertion failed: invariant violated:
<id>`. In the ledger, `EncodeHeader` asserts both invariants on the buffer it produces, and
`DecodeHeader` asserts them again on the buffer it is given. The property is checked once
when the bytes are written and once when they are read back. Tiger does not check that the
two sides agree, so removing one side's assertion still passes `tiger check` as long as the
invariant is asserted elsewhere in production code.

An invariant cannot be pinned: unlike an effect set or frame, it has no computed baseline
for the analyzer to check it against. It is intent the author states, and tiger enforces
the consequences of that statement instead of inferring it. At run time the constant does
nothing more than name the invariant in the failure message. The analyzer does its real
work earlier, when it finds every reference to the named constant across the module. A
bare string typed directly into an assertion call gives it nothing to find.

Tiger checks two things about every declared invariant. It must be asserted by at least
one `assert.Invariant` call outside a `_test.go` file. One site in the module is enough.
Otherwise `tiger check` fails with a finding that offers two fixes: add the call, or
delete the declaration. It must also have a violating test, written with
`assert.Violates(id, fn)`. `assert.Violates` runs `fn` and fails unless `fn` panics with
exactly that invariant's message. A different panic is re-raised rather than swallowed.
Both rules block the build by default. The ledger has a violating test:

```go
func TestShortBufferViolatesSize(t *testing.T) {
	assert.Violates(inv.HeaderSize, func() {
		ledger.DecodeHeader(make([]byte, ledger.HeaderSizeBytes-1))
	})
}
```

Deleting that test and running `tiger check` produces this:

```
$ tiger check ./examples/ledger/...
examples/ledger/inv/inv.go:13:2: TS-A09: no test violates invariant inv.HeaderSize — add a _test.go function that calls assert.Violates(inv.HeaderSize, func() { ... })
tiger: 1 blocking
```

We require the violating test because an invariant no test can violate is either
unreachable or wrongly asserted, and we want to know which. Both rules are decided only by
seeing the constant's declaration together with the assertions and tests written in the
packages that import it, across the whole module. No single package's analysis pass sees
both sides, so only `tiger check`'s finish step reports them, not the golangci-lint plugin.

Naming invariants this way costs a handful of constants and changes three things. The
constants become an accurate list of what the code guarantees, and stay accurate because
deleting the last production assertion fails `tiger check`. Searching for one constant
finds every site that checks it. An invariant added to look thorough costs a declaration,
an assertion site, and a violating test, more work than finding a real one.

### Effects are the fourth oracle

An effect set is the closed lattice of eight kinds of things a function's code can do
besides compute its result, the vocabulary introduced with pins. It is tiger's fourth
oracle, a check that settles one yes-or-no question about a program, the way the compiler,
a test, and an assertion already do. Tiger computes it for every function,
unconditionally, with no annotation required, before any pin is written. An unpinned
function claims nothing about its effects. Purity is a positive declaration, written
`//tiger:effects none`. IO carries a qualifier, `disk`, `net`, `exec`, or `env`, as the
pin section showed.

Go code already has three of these: the compiler, a test, and an assertion. The compiler
decides before any run, but only about types. It settles whether code is well-formed and
well-typed, because that is what compile time means. A test decides about behaviour, the
question the compiler leaves open. It decides only on the inputs its author wrote down, so
it covers exactly the cases someone thought of. An assertion decides on every input that
actually arrives, the gap a test's written-down inputs cannot close. But it decides only
at the point in the code where it sits, at run time, in production as much as under test.
Tiger's specification calls this an oracle that every future execution reuses for free.

None of these three can say what a function is allowed to do. A test or an assertion
speaks only to a path some input has taken. A static type system does not distinguish
functions by what they do at runtime, only by whether their types line up. An effect set
settles what a function's code reaches for, before anything runs, for every input. That
comes from the function's static call graph, rather than from any execution of it. It does
not judge whether a function's effects are appropriate for where it sits, whether a
function in that position ought to block, touch the disk, or spawn. It only checks that
the pin is accurate. Which effects are acceptable is a review judgment, and the pin is
where that judgment is written down.

Tiger builds a function's effect set from what its own instructions do: an allocation, a
channel send or receive, a `go` statement, or a panic. It combines that with the effect
set of every function the code statically calls. A same-package callee contributes its
own computed set or pin, a standard-library callee what a committed table says about it,
and a cross-package callee its exported fact. Tiger composes all of this to a fixpoint, so
mutual recursion resolves.

### One pin holds the whole subtree

A pin on an exported function promises something about every function it calls,
recursively. That reachable set is the subtree, the group tiger checks against the pin,
and the reach is what lets a handful of pins on a package's exported functions enforce
that promise across the whole package.

The reach follows from how tiger computes the effect set a pin fixes. It composes the
pinned function's own instructions with the contribution of every function it calls,
recursively, so a helper several calls down still contributes its effects up to the pin. A
pin such as `//tiger:effects none` freezes that computation at a fixed value: no effects,
for the whole subtree.

A change to any function reachable from a pinned function can fail the pin, even
when the pinned function's own source lines never change. `Serve` is pinned with no
effects. It calls a helper, `dial`, that makes a network call:

```go
//tiger:effects none
func Serve() {
	dial()
}

func dial() {
	net.Dial("tcp", "localhost:0")
}
```

Checking this package produces:

```
$ tiger check ./logger/...
logger/logger.go:5:1: TS-F02: this function makes a network call (app.example/logger.dial at logger.go:7:6) but its //tiger:effects comment doesn't list io(net) — add io(net) to the comment, or remove the call
tiger: 1 blocking
```

The finding is reported at the pin itself, the `//tiger:effects` comment line, never the
line that changed or the function declaration below it. The call it names is the one made
directly inside `Serve`'s own body, `dial`. Had `Serve` instead called `prepare`, which
called `launch` to start a goroutine, the finding would still name `prepare`'s call site
inside `Serve`. Tiger records the first call that introduces the widened effect. Two
fixes are offered: remove the network call, or change the pin to `//tiger:effects
io(net)`.

Pins attach to exported functions and methods only because unexported functions are the
region that mechanical refactoring, extract-function, inline-function, and rename, must
reshape freely. A pin on each would need editing with every reshape. Nothing is lost by
leaving them unpinned: every unexported function is already part of some exported
function's subtree, so its effects are already covered by that pin.

The subtree does not stop at a package boundary. Effect sets cross packages through a
compiler-style fact on each exported function. A pinned function exports its pin as that
fact, and an unpinned one exports its computed set. A caller elsewhere imports the fact
instead of re-deriving the callee's body. An exported `Run`, pinned `//tiger:effects
none`, calls a pinned `app.example/dialer.Open` that promises `io(net)`. That call fails
the same way, naming `app.example/dialer.Open` at its call site. Tiger uses a pinned
callee's pin directly instead of walking its body again. Pinned functions are excluded
from the composition walk, so only the callee's own check against its own pin has to
hold. Checking only the callee's pin instead of its body keeps the checking modular where
pins are dense. Changing `Run`'s pin to `//tiger:effects io(net)` makes the check pass.

### A frame is where a function writes

A function's frame is the set of locations, reachable from its parameters and receiver,
that it writes, transitively through everything it calls. A pinned effect set says what
kinds of thing a function does, such as allocate, block, or read the clock. A pinned frame
answers a different question: which function could have written a value. A frame turns
that question into one query instead of a call-graph read. When a field holds a value it
should not, the candidates are exactly the functions whose frame contains that field. A
function whose frame omits the field could not have written it, however much of the
program that function touches. Tiger computes a frame for every function the way it
computes an effect set, deriving it from the code and its callees from day one with no
annotation required. Only a pin turns that computed baseline into a contract.

A `//tiger:frame` pin is exact in both directions, like an effects pin. It fails on a
write it doesn't list, and it fails on a listed location the function never writes, both
reported at the pin. `Apply` is pinned to `r.log` and calls a helper that also advances
the checkpoint:

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

`Apply` also writes `r.checkpoint` through its helper `advance`, and the check catches it:

```
$ tiger check ./frame/...
frame/replica.go:9:1: TS-F07: this function writes r.checkpoint (at replica.go:14:11) but its //tiger:frame comment doesn't list it — add r.checkpoint to the comment (//tiger:frame r.checkpoint, r.log), or remove the write
tiger: 1 blocking
```

The other direction fails the same way: a pin naming a field the function never writes
gets a finding with the corrected comment.

An unpinned exported function has its frame printed under `--show-facts`, in the syntax
that would freeze it. The absence of a pin is never a finding. `EncodeHeader` writes
nothing through its receiver or parameters, so its frame is empty. The `tiger pin
EncodeHeader` run shown earlier printed that as `//tiger:frame none` beside the effects
line.

Tiger's trace has a known limit: it follows a write back to a receiver or parameter only
through field selections, element addresses, and pointer dereferences, and no further.
The trace does not follow a helper that returns a pointer into that state, code that
copies part of it into a package-level variable, or a write made through an interface
call. Such a write stays out of the frame silently, never as a finding. A frame pin
proves only what tiger can trace. The rule reference records the gap as a known miss
rather than a finding tiger could raise.

### Whole bug classes are excluded, not caught

Tiger does not search for an unbounded loop, a missed switch case, an orphaned goroutine,
or a select that cannot be cancelled. It refuses the shapes that can contain them, so the
bug cannot be written in the first place.

An off-the-shelf linter's automatic rules check code against a fixed pattern. Tiger's
custom analyzers classify every instance of a construct instead: every loop, every switch
over a closed type, every `go` statement, and every blocking `select` is either compliant
with an allowed shape or a finding, and none goes unclassified. Whether an instance is in
an allowed shape has an answer for every instance, where whether an arbitrary loop halts
does not. Four small files below, each written the way a Go programmer would write it
without thinking, show what that classification finds.

```
$ tiger check ./bugs/...
bugs/drain.go:5:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
bugs/status.go:11:2: TS-S08: this switch over Status has no default arm — add default: assert.Unreachable("Status: unhandled value") so a new Status constant fails loudly
bugs/wait.go:4:2: TS-C05: this select blocks with no case that ends the wait on shutdown — add case <-ctx.Done(): return ctx.Err()
bugs/worker.go:4:2: TS-C02: this go statement starts a goroutine nobody owns: nothing says when it exits or ties it to a context — start it through errgroup.Group.Go
tiger: 4 blocking
```

The allowed shape for a loop starts with a stated bound: its `for` condition must be built
from a constant, a `len`, or a counter, and anything else is a finding. No directive can
waive that, except a `//tiger:batched` escape scoped to a cursor-shaped loop. A bound only
promises that the loop could end, not that it ever does. The rule asks only whether such a
bound exists, so a loop testing `len(pending) > 0` without ever shortening `pending` still
satisfies it. Tiger also requires a variant: an expression that strictly decreases every
pass and stays bounded below, the [standard argument](https://doi.org/10.1090/psapm/019/0235771) that a loop terminates. For linear
integer expressions over local variables, it synthesizes that variant unaided, which
covers nearly every real loop. When synthesis succeeds, no pin is needed and nothing
prints. Where synthesis fails, the argument exists only in the author's head. Tiger then
requires a `//tiger:variant <expr>` pin, attached to the loop itself rather than to the
enclosing function. In `bugs/drain.go`, the loop shortens its slice only inside an `if`,
so nothing shrinks on every pass.

A `switch` over a type with a fixed, closed set of values must cover every value and end
in a `default` arm that calls `assert.Unreachable`. Go's compiler cannot check that such a
switch is complete, because to it a constant declared with `iota` is only an integer. The
`default` arm exists to catch a value that arrived from outside the program, off the wire,
from another package, or from stored data, and was never one of the declared constants:
that value reaches `assert.Unreachable` instead of falling through the switch. The rule's
intent is that adding a new value to the set breaks every switch not yet updated for it.
Once every switch ends in `assert.Unreachable`, that is what happens. A type whose values
are meant to grow is marked `//tiger:openenum` in its declaration's doc comment. A switch
over such a type still needs a `default` arm, but there the arm is a legitimate catch-all,
so `assert.Unreachable` is not required. This rule caught three real bugs on the two real
codebases tiger was run against before its rules settled, two of them storage backends
that silently dropped an action because their switch had no default case.

A blocking `select`, one with no `default` case, must carry a case that ends the wait on
shutdown. That case can receive from `ctx.Done()`, or from a channel tiger recognizes as a
shutdown channel: one typed `struct{}`, or named `shutdown`, `stop`, `quit`, or `done`.
Without that case, a shutdown request has no effect, so the wait blocks until something
else ends it, such as `SIGKILL`. A process killed mid-write loses the write. This rule
caught a real bug in those runs: a queue's Produce, Complete, and Retry waits could never
be cancelled, while its documentation claimed a cancellation that only its Lease method
actually implemented.

The allowed shape for a goroutine is to start it through a supervisor, a goroutine-owning
pattern such as `errgroup.Group.Go`, which ties the goroutine to a context and to code
that waits for it to finish. A bare `go` statement anywhere else is a finding, because
nothing in it says when the goroutine exits or ties it to a context. Tiger keeps no
allowlist of supervisor functions, because listing one would exempt every `go` statement
written inside it. Starting a goroutine also puts `spawn` in the function's effect set,
so a `go` statement added several calls beneath a pinned function fails that pin on the
commit that added it. The same runs turned up a real bug from this rule: a daemon's
`sync.WaitGroup` went unwaited, so its `Shutdown` method could return before its HTTP
goroutines had exited.

### Rules and the two severities

A rule's severity is one of exactly two values, blocking or advisory, set once in the rule
registry and never inside the analyzer that emits the finding. Blocking is the default. A
finding fails the run unless that registry entry says advisory instead. There is nothing in
between. A rule either blocks or it is not a rule, because warnings are golangci-lint's
job.

Advisory severity covers escape directives, skipped tests, and two style rules whose check
is a heuristic, struct declaration order and a defer placed far from what it releases. None
of these fails the run by itself. Each is counted against a per-package budget instead.

### The budget and the ratchet

The budget exists for two things tiger allows on purpose: `//tiger:batched` escape directives and skipped tests. Neither fails the run by itself, so without a limit a package can carry any number of them. The budget is that limit, one row per package per advisory rule in `tiger.budget.yaml`, committed with the code. A package with more escape directives or skipped tests than its row for that rule allows fails the run. The two advisory style rules are counted the same way, and a blocking finding is never counted: it fails the run regardless of the file.

The tool only lowers the numbers. `tiger budget --write` creates a row for a package that
has none, lowers a row when the count drops, and deletes the row at zero. It has no
operation that raises a row, and we call that the ratchet. Tiger's primary user is an AI agent. If a raising command existed, an agent whose package is over its number would run it and change no code. The only way to raise a number is to edit the file by hand, and the edit is then
a line in the pull request diff. A reviewer sees it there. A global threshold gets raised
at 2am before a release, and a ratchet cannot be raised without a commit that says so.

Only the tiger CLI enforces the budget. Under the golangci-lint plugin every escape
directive and skipped test reports as an ordinary issue, because the plugin has no
end-of-run step that totals counts across packages.

### Directives

A directive is a `//tiger:<verb>` comment that lets code carry a claim tiger can check: a
pinned fact, a declared intent, or an escape from a rule, and it binds to the code on the
line after its comment group. The verb vocabulary is closed and owned by a single package,
which also formats every directive. A finding's text and the pin that satisfies it are
identical by construction. An unknown verb, or an escape verb with no reason after it, is
a blocking finding.

There are three kinds of directive.

1. Pins freeze a computed fact into a contract, and drift from it afterward is a blocking
   finding. Two more verbs join `//tiger:effects`, `//tiger:frame`, and `//tiger:variant`
   here: `//tiger:requires` and `//tiger:ensures`, which pin a function's contract.
   `requires` states a precondition the caller must establish, and `ensures` states a
   postcondition the function guarantees on return. Each is written as a small comparison
   such as `n > 0` or as an invariant ID. `tiger pin` writes effects, frame, and variant
   pins. It does not write requires or ensures.
2. Intent declarations state something no analyzer can compute, and the code is held to it
   afterward. `//tiger:openenum`, introduced earlier, is the one verb of this kind.
3. The one escape directive, `//tiger:batched`, loosens one rule at one site and always
   carries a reason. It counts against its package's budget on every run, whether or not
   it is currently waiving a finding.

An escape is admitted only where the rules eliminate something reality requires and no
rewrite satisfies the rule instead. The primary consumer of tiger's findings is an AI
agent, and an agent offered a cheaper path than conforming will take it. `//tiger:batched`
was admitted because some external systems accept only one item of IO at a time. No
rewrite of the caller changes that. There is deliberately no directive that dismisses a
rule at a site. The only ways past a finding are a satisfying rewrite or a reasoned escape
counted against the budget.

## What tiger tells a reviewer

The check and the pins tell a reviewer something about a change before the reviewer reads
any of it. A pin whose effect set leaves out `io` is a promise that the function does no
IO, and the check holds it to that promise on every run. If the agent adds a database
call to a method pinned that way and leaves the pin alone, the check fails at the pin.
The finding names the call that introduced the effect. If the agent widens the pin
instead, the check passes, but the edited pin line is in the diff. Either way the reviewer
learns that the method now talks to the database without reading its body. They open the
diff already knowing what changed, and read it to learn why.

Tiger reduces the need to read the code but does not remove it. A change that passes the
check and edits no pin is still reviewed, often by other AI agents. No rule decides
whether the logic is right. The check has already settled four things about such a
change: no new effect, no new write outside the pinned frame, no declared invariant left
unasserted, no loop without a bound. What is left for the reviewer is the part of the
change that can still be wrong.

## What tiger will not do

Tiger refuses four things on purpose: a severity between blocking and advisory, output
that reassures, a suppression that carries no reason, and a noisy rule kept for its good
intentions.

Tiger has two severities for its own findings, blocking and advisory, with nothing between
them. A third tier called reported once existed in the rule registry, the command-line
interface, the editor plugin, and the specification. We deleted it outright, and no future
rule can use it.

The reason is a division of labor: a finding a reader may weigh and set aside is a warning.
Warnings belong to golangci-lint's half, not to tiger's own rules.

A clean run of `tiger check` exits 0 and prints nothing, no success banner and no
informational line. Passing `--show-facts` to a clean package prints two fact lines. The
run still exits 0, since a fact never changes the verdict. An advisory finding inside its
package's budget is silent, not quieter. On a package with one escape directive, that
finding appeared inside a failing run with no budget row printed. Once `tiger budget
--write` recorded the count, the next run printed nothing.

Tiger refuses the suppression that carries no reason: an escape directive is never a bare
suppression. Tiger checks that its verb is recognized and a reason string is present, not
that the reason is true. The directive counts against its package's budget on every run
for as long as it exists.

When a rule's output on real codebases turns out dominated by findings a reader would
decline to act on, we remove it rather than leave it at advisory or keep tuning its
threshold. Removal is complete: the analyzer, its test corpus, registry entry, and the
specification's enforcement line are all deleted together. No analyzer is disabled by
default, and nothing marks where it used to be. Where the maxim, the principle a rule was
meant to enforce, still holds and only the mechanical check failed, the specification
keeps the rule's text and names human review as its enforcement.

Three rules about names were removed after the runs on two real codebases found their
combined output dominated by the codebase's own domain vocabulary and API conventions: 58
percent of the remaining advisory findings in one codebase, 35 percent in the other. One
flagged vague name tokens, one flagged a name that echoed its own type, and one paired
names across a conversion boundary. We withdrew two more rules to review after a later
audit. One had asked a single-caller helper to carry its caller's name, and its suggested
renames compounded down a call chain into names like `checkEventLoopSelectHasTerminationCase`.
The other flagged a variable declared more than ten lines before its first use. It could
not tell a variable that has to sit above the loop it accumulates into from one that was
misplaced. Both analyzers were deleted, and both rules' text stayed in the specification,
enforced now by review.

A run over tiger's own source tree, with zero blocking findings, printed 324 advisory
lines before these removals and none after. The same removals left two real codebases with
advisory output that was mostly the two standing notices the channel exists for. A rule
whose output is mostly noise teaches a reader to stop reading the channel it prints on. A
reader who has stopped reading also misses the notices the channel exists for. Holzmann
saw the same thing on the Curiosity flight software. Developers ignored a module that
showed a thousand warnings, so he [filtered the
view](https://www.youtube.com/watch?v=GRJtYwneG2Q) to ten or twenty per module, and raised
the filter as those were fixed, until the count reached zero.

## Where tiger stands today

Tiger installs as a single Go binary with `go install`, naming the CLI's module path.
The `check` subcommand runs it against a package tree:

```
$ go install github.com/kapetan-io/tiger/cmd/tiger@latest
$ tiger check ./...
```

A first run against a codebase tiger has not seen before produces many findings, because
tiger checks the whole tree against every rule on that pass. We saw 1,023 blocking
findings against a queue codebase we knew and trusted, about 36,000 lines of code plus
about 6,000 of tests.

Every one of those findings is blocking. Each rule exists to prevent one bug class its
specification entry names, and there is no warning tier. A finding is a true positive
to fix or a false positive that is a bug in tiger. Filing that bug adds the case
to the rule's test corpus, so it cannot regress.

The way to judge tiger is to read a sample of its findings against your own judgment. If
most name something you would want fixed, gating CI on `tiger check` is warranted.

The repository's [examples/ledger](../examples/ledger) is built to the invariant pattern:
an `inv` package declaring the invariant IDs, an encode and decode pair asserting them,
and a violation test per invariant. It passes `tiger check` with zero findings today, a
live example of code that satisfies every rule tiger enforces, not just the invariant
ones.

The [rule reference](Tiger%20Rule%20Reference.md) gives one entry per enforced rule: what
it requires, why it exists, a firing example, and the compliant rewrite. A doc meta-test
in `internal/rules` fails when the reference and the rule registry disagree. The
[specification](Tiger%20Specification.md) is the normative document the reference is
drawn from.

---

Every transcript on this page is live `tiger check` output. The `EncodeHeader` runs and
the invariant run are against `examples/ledger`. The two `Drain` runs, the `Serve` and
`dial` run, the `Apply` and `advance` run, and the four-file `bugs` run are against
scratch packages, since the ledger has no function with helpers beneath it and no loop,
switch, goroutine, or wait written in the shapes tiger rejects. The `tiger golangci` audit
is against a config `tiger golangci --init` generated, with one setting changed by hand.
The `Clock` and `Prune` snippet under "What a surface is" is illustrative and not in the
ledger; the `Expire` and `ExpireDirect` run is real, against a scratch package that also
declares two implementations of `Clock`, which the code block omits.

## References

1. Gerard J. Holzmann. "The Power of 10: Rules for Developing Safety-Critical Code." *IEEE Computer* 39(6), June 2006, pp. 95–99. [doi:10.1109/MC.2006.212](https://doi.org/10.1109/MC.2006.212). Author's PDF: <https://spinroot.com/gerard/pdf/P10.pdf>.
2. Gerard J. Holzmann. "Mars Code." *Communications of the ACM* 57(2), February 2014, pp. 64–73. [doi:10.1145/2560217.2560218](https://doi.org/10.1145/2560217.2560218). How the rules were applied to the Curiosity flight software.
3. Gerard J. Holzmann. "The Power of Ten: Rules for Safety Critical Coding." Talk at Systems Distributed '26, Boston, July 27, 2026. <https://www.youtube.com/watch?v=GRJtYwneG2Q>.
4. Jet Propulsion Laboratory. *JPL Institutional Coding Standard for the C Programming Language.* JPL D-60411, version 1.0, March 2009. Mirror: <https://everyspec.com/NASA/NASA-JPL/JPL-D-60411_VER-1_32832/>.
5. Gerard J. Holzmann. Cobra, a static source-code analyzer. <https://spinroot.com/cobra/>, source at <https://github.com/nimble-code/Cobra>.
6. Gunnar Kudrjavets, Nachiappan Nagappan, and Thomas Ball. "Assessing the Relationship between Software Assertions and Faults: An Empirical Investigation." *Proceedings of the 17th International Symposium on Software Reliability Engineering (ISSRE 2006)*, pp. 204–212. [doi:10.1109/ISSRE.2006.14](https://doi.org/10.1109/ISSRE.2006.14). Technical report: <https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2006-54.pdf>.
7. Thomas J. McCabe. "A Complexity Measure." *IEEE Transactions on Software Engineering* SE-2(4), December 1976, pp. 308–320. [doi:10.1109/TSE.1976.233837](https://doi.org/10.1109/TSE.1976.233837). The source of the cyclomatic complexity limit of 10.
8. G. Ann Campbell. "Cognitive Complexity: A New Way of Measuring Understandability." SonarSource white paper, 2018. <https://www.sonarsource.com/docs/CognitiveComplexity.pdf>. The metric gocognit implements.
9. Robert W. Floyd. "Assigning Meanings to Programs." *Proceedings of Symposia in Applied Mathematics* 19, 1967, pp. 19–32. [doi:10.1090/psapm/019/0235771](https://doi.org/10.1090/psapm/019/0235771). The origin of the loop variant as a termination argument.
10. MISRA. *MISRA C: Guidelines for the Use of the C Language in Critical Systems.* <https://misra.org.uk/product-category/misra-c-2/>. The safety-critical standard Holzmann surveyed before writing the Power of Ten.
11. TigerBeetle. *TigerStyle.* <https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md>.
12. TigerBeetle. *The VOPR*, the deterministic simulator. <https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/internals/vopr.md>. See also the safety overview at <https://docs.tigerbeetle.com/concepts/safety/>.
13. Jingyu Zhou et al. "FoundationDB: A Distributed Unbundled Transactional Key-Value Store." *Proceedings of the 2021 International Conference on Management of Data (SIGMOD '21)*, pp. 2653–2666. [doi:10.1145/3448016.3457559](https://doi.org/10.1145/3448016.3457559). Deterministic simulation as the primary test method for a distributed database.
14. Will Wilson. "Testing Distributed Systems with Deterministic Simulation." Talk at Strange Loop 2014. <https://www.youtube.com/watch?v=4fFDFbi3toc>.
15. Gary Bernhardt. "Boundaries." Talk at SCNA 2012. <https://www.destroyallsoftware.com/talks/boundaries>. Companion screencast, "Functional Core, Imperative Shell": <https://www.destroyallsoftware.com/screencasts/catalog/functional-core-imperative-shell>.
16. Alistair Cockburn. "Hexagonal Architecture." 2005. <https://alistair.cockburn.us/hexagonal-architecture>.
