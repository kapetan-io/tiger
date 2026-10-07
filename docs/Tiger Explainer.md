`tiger check` · binary verdict · change detection

**This page describes tiger as it is meant to be when it is finished.** It reflects the
decisions in ENG-191, and several of them are not built yet: the `tiger changed` report, the
patterns collection, the loop, goroutine, and wait rules as described here, and the
collect-then-sort part of the map-order rule. Each section that describes unbuilt behavior says
so. Every transcript on the page is real output from today's `tiger check`, except where a
section names the prototype it came from, so a finding's wording can differ from the finished
tool's. The rule reference lists every rule tiger enforces today, and the specification marks
each rule that is not yet built.

## Overview
Tiger is a static analyzer for Go, built for code that AI agents write. It holds that code to the restrictions NASA's [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) and TigerBeetle's [TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) put on flight and database software, so the result is testable and free of whole classes of production bugs. Every loop has a bound, every goroutine has an owner, and every clock, random source, and IO call is passed in, so any function can be tested in isolation. A run either passes or fails. Tiger has no warnings and no suppression comment, so code that fails the check is rewritten until it passes. The analysis is deterministic static analysis and never a language model, so the same code gets the same verdict every time.

Tiger also has a second command, `tiger changed <base>`, which compares the code at a base revision such as `main` with the working tree. If a program is only supposed to reach the database through a `Store` interface, and an agent adds a database call somewhere else, the report shows the new call, so the reviewer learns that the pull request changed the architecture and not only the code it touched. For each struct, or each free function, it lists the types and functions it calls now that it did not call at the base. `tiger changed` is not a rule, so it never fails the run. It is a report, meant to be read by a human or an agent.

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

**Tiger reports architectural change**
Tiger builds the report from the code at both revisions, so a reviewer sees each new call a change makes before reading the diff.

- **A change is reported on the struct where it was written,** not on everything that
  calls that struct.
- **A call through an interface is named by the interface.** Tiger does not guess which
  implementation runs underneath it.
- **Pure helpers are left out.** Calls into packages that never reach the operating
  system, such as `strings`, `sort`, and `fmt`, are not reported.

**Tiger has no warnings**
AIs (like humans) can get lazy, and often just want to complete the task instead of writing quality, maintainable code. `tiger check` has no suppression comment and never honors `//nolint`, so the only way past a finding is a rewrite. Holzmann's [tenth rule](https://spinroot.com/gerard/pdf/P10.pdf) is the same: compile with all warnings on and allow none, because [one new warning among thousands is invisible](https://www.youtube.com/watch?v=GRJtYwneG2Q).

- **The agent can't bypass the check.** A run fails or it passes. Tiger has no
  warnings and no suppression comment, so the agent's only path forward is a
  rewrite.
- **Human review should be about behavior and intent, not code.** If a change passes your test suite and the check, the human reviewing the PR can focus on intent and behavior instead of code.

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
- **The dependency graph is written down.** A checked-in layer file says which packages may
  import which, and golangci-lint's depguard fails an import that goes against it. Layering
  is the part of architecture a tool can enforce.
- **Goroutines follow one set of rules.** Every goroutine has an owner and a context that
  ends it. A package shares state through channels or through a mutex, never both, and no
  wait blocks without a way to cancel it.
- **Interfaces say what they mean.** Exported functions take domain types, not bare
  strings and integers, so a value nobody checked cannot reach them.

## golangci-lint and tiger's own analyzers
Tiger's rules split in two by what checks them. Some of them can be checked by a linter
that golangci-lint already ships, such as funlen for function length. For those, tiger does
not write its own check. It generates the golangci-lint configuration that turns each
linter on at the setting the rule requires, audits that configuration against the rules,
and leaves the checking to golangci-lint. The rest of the rules no existing linter checks,
so tiger carries analyzers of its own for them, run by `tiger check`. The specification
calls the first group the auto half and the second the custom half.

Among the rules tiger leaves to golangci-lint:

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
many issues it reports. `tiger check` has no suppression comment and does not read
`//nolint`. Under golangci-lint, `//nolint` works as it does for any linter, including on
tiger's analyzers when they run as a plugin. That is why `tiger check` is the verdict, and a
project that wants no `//nolint` at all sets `nolint.forbid` in `tiger.yaml`, which makes every
`//nolint` in a checked package a blocking finding under `tiger check`. Tiger's one escape
directive, `//tiger:batched`, is counted against a per-package budget on every run. CI runs
golangci-lint with the generated config and `tiger check ./...`, and both must pass.

Tiger keeps no baseline file of existing findings, and it does not recommend golangci-lint's
`--new-from-rev` flag, which reports only findings on lines a branch changed. A finding on an
old line is still a finding, so an existing codebase adopts tiger by fixing what it reports.

The custom analyzers check shapes no linter models: whether every loop has a bound and a
proof that it ends, whether every goroutine has an owner that waits for it, whether every
blocking wait can be cancelled, whether a map range can leak its order, and whether every
declared invariant is asserted and violated by a test. A few rules are
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
the code, run the check, fix what it finds, then read what the change starts calling. A run that finds
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

### 2. Read what the change starts calling

`tiger changed <base>` lists, for each struct or each free function, the types and
functions the code now calls that it did not call at the base. It compares that base revision against the working tree.
`tiger changed main`, for example, uses main as that base.

On querator commit aeb014c, where the daemon starts constructing its Postgres stores,
`tiger changed` prints this:

```
daemon.setupPartitionStorage
  + internal/store.NewPostgresPartitionStore: setupPartitionStorage calls NewPostgresPartitionStore
```

The report attributes a change to the struct where it was written, not to every struct
that calls it. If a struct B starts making the call and a struct A only calls B, only B
is reported. Three more rules decide where a call shows up:

- A new method on a type the struct already calls is not reported, but a new
  constructor, or a new type, is.
- Work handed over a channel shows on the struct at the receiving end, once that struct
  makes the new call.
- A call made through an interface is named by the interface itself, never by a guess
  at the implementation.

Pure helpers are filtered out by the import graph, with no list of packages. A method or
an interface counts when its package reaches `syscall`. A free function counts when its
own body does. The filter keeps `os`, `net`, `time`, pgx, and badger, and drops
`strings`, `sort`, and `fmt`. The report also skips generated files, and it shows
`context.Context`.

That filtering changes what the report shows in practice. On ten querator commits, the
prototype printed five lines, two of them real changes. On git-server's head commit it
printed 173 lines, 110 of them from generated protobuf code. Skipping generated files
and applying the filter brought that down to 36 hand-written lines.

Each known blind spot is covered by a test in the prototype:

- A write made through `io.Writer` is not reported, because the `io` package itself
  never reaches the operating system.
- Moving a call into a separate free function reads as one call removed and one added.
- Renaming a type reads as all of its calls removed and re-added.

The command itself is not built yet, so everything shown above, the transcript and the
counts, comes from a prototype in ENG-191's experiments. Because of that, the
report's exact format is still being settled.

## Key Concepts in Tiger

### Nondeterminism is injectable

Tiger's rule, TS-T01, is that core logic is deterministic and time, randomness,
and IDs are injected. A function that needs the clock, randomness, or IO takes it
through an interface, so production hands in the real implementation and a test
hands in something else. A function is deterministic when the same inputs always
produce the same result. Nondeterminism comes from reading time, randomness, IO,
scheduling, or identity, so the same call can return a different result from one
run to the next. No test can control an input it cannot hand in, and a run that
cannot be controlled cannot be replayed exactly on failure.

`Expire` reads the clock through the interface it was given, and `ExpireDirect`
reads it directly.

```go
type Clock interface {
	Now() time.Time
}

func Expire(clock Clock, deadline time.Time) bool {
	return clock.Now().After(deadline)
}

func ExpireDirect(deadline time.Time) bool {
	return time.Now().After(deadline)
}
```

golangci-lint checks this, in the configuration tiger generates. The forbidigo
linter forbids direct calls to `time.Now`, `time.Sleep`, and `time.After`, and
to `panic`, so `ExpireDirect` is reported and `Expire` passes. The depguard
linter forbids importing `math/rand`, `math/rand/v2`, and `crypto/rand` in a
package under `internal/domain`. Both linters check calls and imports written in
the module's own code, so a third-party library that reads the clock internally
is not caught.

The coder decides whether a test hands in a substitute or the real dependency,
per test. For a crypto library, the real one is usually the better choice. Tiger
requires no fake, mock, or simulation to exist. Where a coder writes a substitute
for fault injection, review judges which faults it injects, such as a torn
write, a misdirected write, or an fsync that lies. No analyzer checks that
judgment.

Once a test supplies a clock and a store it controls, every result downstream is
a function of the test's inputs and those two answers. Handing in the real clock
or the real disk instead makes the run only as repeatable as that dependency.

Map iteration order is a separate source of nondeterminism. Go randomizes the
order it visits a map's entries on every run. That order is not a value passed
into a function, so there is nothing to inject. Tiger checks it with a different
rule, TS-T02, the `maporder` analyzer, which bans every range over a map unless
the loop body matches one of a fixed set of shapes that cannot depend on order,
such as summing, counting, writing into another map, or collecting into a local
slice that is sorted before any other use.

A collect-then-sort passes when the sort is `sort.Strings`, `sort.Ints`, or
`slices.Sort` on string or integer elements, because two equal strings or
integers cannot be told apart. Floats are excluded, because -0 and +0 compare
equal and print differently. A comparator sort passes only when its comparisons
together cover every field of the element, using `cmp.Compare`,
`strings.Compare`, or `bytes.Compare` on a whole array. Covering every field
means the comparator returns 0 only for identical elements, so no tie can leak
map order. A result of `maps.Keys`, `maps.Values`, or `maps.All` must go to
`slices.Sorted`, to a sort whose comparator covers every field, to
`maps.Collect`, to `maps.Insert`, or to a range checked the same way a map range
is, so `slices.Collect(maps.Keys(m))` is a finding.

A finding means the loop's shape is outside the allowlist. Tiger does not check
whether the output varies from run to run. Five shapes pass despite depending on
order. One is last writer wins (`inverse[v] = k` when two keys share a value),
where whichever key is visited last overwrites the other in the result. The
others are a map write whose value calls a function (`labels[k] =
fmt.Sprint(v)`), `golang.org/x/exp/maps.Keys`, `reflect.Value.MapKeys`, and a
range over a type parameter constrained to a map. TS-T11 runs the test suite
twice and diffs the output, which catches an order leak on any path the tests
exercise. The collect-then-sort and full-coverage parts of TS-T02 are decided
but not yet built.

Deterministic simulation found the bugs that made TigerBeetle worth imitating.
Their simulator, the
[VOPR](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/internals/vopr.md),
runs the real consensus and storage code in one process with simulated time,
network, and disk faults, a technique
[FoundationDB](https://doi.org/10.1145/3448016.3457559) described in
[2014](https://www.youtube.com/watch?v=4fFDFbi3toc). Injection makes such a
simulator possible. This discipline is TigerBeetle's. NASA's Power of Ten has no
determinism rule.

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

An invariant has no computed value for the analyzer to check it against. It is intent the author states, and tiger enforces
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

TS-S02 requires a loop's header to state a bound, built from a constant, a `len`, or a counter. A counter counts as a bound only when its comparison is a top-level `&&` term in the condition, not inside `||` or behind a `!`. It must also pair `<` or `<=` with `++` or `+=`, or `>` or `>=` with `--`, never `!=`. Nothing in the loop body may write to the counter or its limit, whether through a helper function, a method, a pointer, or an insertion into a map whose length is the limit. The limit must also stay below the counter type's maximum value.

A bound says only that the loop could end, not that it does. TS-V01 also requires a variant, an expression that strictly decreases on every pass and stays bounded below, [the standard argument for why a loop terminates](https://doi.org/10.1090/psapm/019/0235771). Tiger synthesizes this variant itself for several familiar shapes.

- a fixed step such as `i--`, `i += 2`, or `s = s[1:]`
- division by a constant above 1, or a shift by a constant of at least 1 under a `> 0` condition, as in `size >>= 7`
- slicing by the length of a part tiger can show is not empty, as in `data = data[len(chunk):]`
- a struct field tested in the condition when no call in the body receives the struct
- an `&&` condition where one side shrinks

Nothing prints when synthesis succeeds.

Where tiger can't synthesize one, tiger asks for it directly, as a `//tiger:variant <expr>` comment attached to the loop itself. The expression must be small arithmetic, a number, a variable, or a `len(...)` call, and tiger checks it against the same rules it uses for synthesis, so a variant that doesn't hold is a blocking finding. TS-S02 accepts a loop whose variant tiger verified, whether tiger found it unaided or a developer wrote the comment. In `bugs/drain.go`, the loop shortens its slice only inside an `if`, so nothing shrinks on every pass.

When no variant exists, the way out is a counter used as a cap. A cap may stop without asserting only when its limit is not a constant and every other way out of the loop drains a stream whose type is declared outside the module, such as `rows.Next()` or `scanner.Scan()`. There the limit is a page size, and reaching it means the page is full. Any other exit, an internal condition or a `break`, makes the counter a safety cap, so the loop must assert or return an error when the counter reaches its limit. A safety cap that stopped silently on a cycle would return a truncated result as though it were complete.

Every loop limit is also checked at its source. Tiger traces the limit backward through parameters, struct fields, and return values, and every value written into it must be a constant, a length, another bounded limit, or a value clamped against a declared maximum, such as `min(config.Spins, spinsMax)`. A prototype of this check already found three bugs that no rule in tiger today reports.

- git-server allocated 352 MB for a 12-byte push whose pack header claimed 4 million objects
- six of querator's seven list endpoints accepted any page size up to 2^31−1 from the request
- querator's create-queue endpoint accepted any number of partitions and looped once per partition

A few loop shapes have fixed answers. Ranging over a standard-library iterator such as `maps.Keys` counts as bounded, while ranging over any other iterator is a finding, the same as ranging over a channel. A `for` loop testing `ctx.Err() == nil` against a `context.Context` parameter counts as running until cancelled, the same as a `select` on `ctx.Done()`. Testing `context.Background()` does not.

A loop pulling rows from a store cursor restates the limit its query already declared, as in `for count := 0; count < opts.Limit && rows.Next(); count++`. A loop that filters rows before keeping them needs two separate limits instead, a page size that counts the rows it keeps and a scan maximum that counts every row it reads, returning an error if that maximum is reached. These changes to the loop rules are decided but not yet built, so today's tiger still accepts some loops that run forever.

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

TS-C05 requires a blocking select, one with no `default` case, to carry a case that can end the wait on shutdown. TS-S03 requires the same of a loop built to run forever. The accepted stop cases are a receive from `ctx.Done()`, where the context does not trace, within the function, back to `context.Background()` or `context.TODO()`, or a receive from a channel that the owning package itself closes or sends on. When that channel is a struct field, the closing or sending can happen in any method of the owning type that an exported method can reach. A channel named `shutdown` that nothing in the package closes is a finding. A second rule covers the loop around the select. An event loop's only way out must be that stop case, so no `return` or `break` elsewhere in the loop may leave it. Without a stop case, a shutdown request has no effect, so the wait blocks until something else ends it, such as `SIGKILL`. A process killed in the middle of a write loses that write. TS-C05 caught a real bug, a queue whose Produce, Complete, and Retry waits could never be cancelled, though its documentation claimed a cancellation that only its Lease method implemented.

TS-C02 requires a goroutine to start only through `errgroup.Group.Go` or `sync.WaitGroup.Go`, so a bare `go` statement anywhere else is always a finding. The group's `Wait` must be reached on the success path, either in the same function or, when the group is held in a struct field, in an exported method of the owning type. A `Shutdown` method that returns early because its own context has already expired is exempt from that requirement. Tiger keeps no allowlist of supervisor functions, because naming one would exempt every `go` statement written inside it. TS-C02 caught a real bug, a daemon whose `sync.WaitGroup` went unwaited, so its `Shutdown` method returned before its HTTP goroutines had exited.

TS-C03 adds a check on the package as a whole, requiring a `goleak` check in the package's `TestMain`. Tiger blocks a package that starts goroutines through `go`, `wg.Go`, or `errgroup.Go` and has no goleak harness.

The messages for TS-S03 and TS-C02 both point to the shutdown-loop pattern, described later in this document. The pattern does the following.

- sets a flag
- closes a stop channel once
- gives the loop no exit but that stop case
- has the loop close a `done` channel
- starts it with `wg.Go`
- has every waiter select on `done`

These changes to the goroutine and wait rules are decided but not yet built. Today's TS-C02 accepts only `errgroup.Group.Go`, and today's TS-S03 and TS-C05 still accept a channel by its name.

### A pattern covers the part a rule cannot see

A pattern is a small Go package that shows the right shape next to the broken
shape it replaces, with tests that demonstrate both. It covers the part of a
correct shape no rule can check. The broken shape still passes `tiger check`,
since the rule cannot see what is wrong with it, and its test is what shows the
bug.

Some correct shapes tiger's rules can check only in part. The shutdown loop is
one. Tiger can check that the loop's goroutine starts, that its only exit is the
stop case, and that the stop case is a real context or a channel the package
closes. It cannot check that the loop runs its cleanup before it closes `done`,
that `Shutdown` lets the caller's context bound only the wait, or that every
caller waiting on the loop's reply also selects on `done`. Querator's shutdown
bugs, ENG-160 and ENG-196, lived in exactly those parts.

A pattern is added only where a tiger rule stops short and a real bug lived in
that gap, such as a trial finding, a filed ticket, or a bug an experiment
reproduced. A shape that is merely good Go, with no rule behind it and no bug,
does not qualify, which keeps the collection from growing into a general style
guide.

Each pattern names the rules it backs and the part of the shape those rules
cannot see, and turns that part into a short checklist a reviewer can answer yes
or no against a diff. The shutdown loop backs TS-C02, S03, and C05, and its
checklist asks whether every caller waiting on the loop's reply also selects on
`done`, whether `Shutdown` closes stop before it waits, and whether a test calls
`Shutdown` with an expired context.

Patterns live under `patterns/<name>/` in tiger's root module. CI runs `tiger check`
on every pattern, failing on any finding, and runs its tests under `-race`, so a
pattern cannot fall out of date with the rules.

The `tiger` binary embeds the patterns. `tiger pattern <name>` prints the pattern's
card, the rules it backs, the gap those rules miss, the bug behind it, and the
reviewer's checklist, along with the pattern's code and its tests. `tiger pattern
--rule <code>` lists the patterns behind a given rule, and `--checklist` prints
only the reviewer's questions. A release ships this binary alongside the
golangci-lint plugin.

Each rule's registry entry names the patterns behind it, and every finding that
rule reports ends with a pointer such as `see tiger pattern shutdown-loop`, so a
rule violation leads straight to the right shape. A meta-test fails the build if
a pointer names a pattern that does not exist, or a pattern names a rule that
does not exist. Because the pattern ships inside the same binary as the rule, the
pattern an agent reads always matches the rule that flagged its code.

A pattern is not meant to be imported as a dependency. It exists to be followed.

The patterns decided so far are these eight, named with the rules each backs.

- The shutdown loop (stop and done, one exit), backing TS-C02, S03, and C05.
- The goroutine owner (`wg.Go` and `Wait`), backing TS-C02.
- Pool reuse (zero before `Put`), backing TS-M05.
- A capped worklist that reports when the cap is hit, backing TS-S02 and S01.
- A page over an outside stream, backing TS-S02.
- Clamping a limit where it enters the program, backing TS-S02.
- Sorted map output when the key has no order, for pointer or interface keys,
  backing TS-T02.
- A filtered page with a page limit and a scan limit, backing TS-S02.

The repository's `examples/ledger` joins them as the invariant pattern, backing
TS-A07, A08, and A09.

The collection is decided and not yet built. Today the repository has only
`examples/ledger`.

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
pin, a declared intent, or an escape from a rule, and it binds to the code on the
line after its comment group. The verb vocabulary is closed and owned by a single package,
which also formats every directive. A finding's text and the pin that satisfies it are
identical by construction. An unknown verb, or an escape verb with no reason after it, is
a blocking finding.

There are three kinds of directive.

1. Pins state a claim tiger proves, and a pin the code does not satisfy is a blocking
   finding. `//tiger:variant`, introduced with loops, is one. `//tiger:requires` and
   `//tiger:ensures` pin a function's contract: `requires` states a precondition the caller
   must establish, and `ensures` states a postcondition the function guarantees on return.
   Each is written as a small comparison such as `n > 0` or as an invariant ID. `tiger pin`
   writes a variant tiger found as a `//tiger:variant` comment. It does not write requires
   or ensures.
2. Intent declarations state something no analyzer can compute, and the code is held to it
   afterward. `//tiger:openenum`, introduced earlier, is the one verb of this kind.
3. The one escape directive, `//tiger:batched`, waives the rule against IO inside a loop
   body (TS-M10) at one site and always carries a reason. It counts against its package's budget on every run, whether or not
   it is currently waiving a finding.

An escape is admitted only where the rules eliminate something reality requires and no
rewrite satisfies the rule instead. The primary consumer of tiger's findings is an AI
agent, and an agent offered a cheaper path than conforming will take it. `//tiger:batched`
was admitted because some external systems accept only one item of IO at a time. No
rewrite of the caller changes that. There is deliberately no directive that dismisses a
rule at a site. The only ways past a finding are a satisfying rewrite or a reasoned escape
counted against the budget.

## What tiger tells a reviewer

A change that passes `tiger check` has settled several things before the reviewer reads any of it.

- Every loop has a bound, a proof that it ends or a cap that reports when the cap is hit.
- Every goroutine starts through an owner that waits for it to finish.
- Every blocking wait can be cancelled.
- Every switch over a closed type covers every value the type can take.
- Every declared invariant is asserted somewhere in production code and violated by a test that proves the assertion fires.
- Outside a short list of known misses, no map range has a shape that could leak map order.

What the check settles says nothing about whether the change does something new. `tiger changed <base>` answers that question before the diff is opened. For each struct, it lists what that struct calls now that it did not call at the base commit, so when an agent adds a database call inside a handler, the handler's struct appears in the report with the new call. The reviewer learns that the change reaches the database before reading a single line of the diff. They open the diff already knowing what changed, and read it to learn why. A write through `io.Writer` is not reported.

For the part of a shape the check cannot see, each pattern gives the reviewer a checklist. It is a short list of yes-or-no questions, answered by comparing the diff to the pattern's shape, covering the gap no rule can check.

Tiger reduces how much of the code a reviewer needs to read, but it does not remove the need to read it. A change that passes the check is still reviewed, often by other AI agents, because no rule in tiger decides whether the logic is right.

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
informational line. An advisory finding inside its
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
tiger checks the whole tree against every rule on that pass. Today's tiger prints 401
blocking findings against querator, a queue service of about 36,000 lines, and 339 against
git-server, a git server of about 17,000 lines. Both are codebases we knew and trusted.

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

Every transcript on this page is live output from today's tiger unless the section names
a prototype. The invariant run is against `examples/ledger`. The `Drain` run and the
four-file `bugs` run are against scratch packages, since the ledger has no loop, switch,
goroutine, or wait written in the shapes tiger rejects; their messages are today's and will
change as the decided loop and goroutine rules are built. The `tiger golangci` audit is
against a config `tiger golangci --init` generated, with one setting changed by hand. The
`tiger changed` output is from the prototype in ENG-191's `experiments/changed`, run on
querator. The `Clock` and `Expire` snippet is illustrative.

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
