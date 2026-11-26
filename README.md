# Tiger Go

A static analyzer that holds Go written by AI agents to the rules NASA and TigerBeetle wrote for flight and database software.
Every loop has a bound, every goroutine starts through a group that waits for it, and code takes
the clock as an input instead of reading it.

```
go install github.com/kapetan-io/tiger/cmd/tiger@latest
```

```
> tiger check ./...

newton.go:7:2: TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } ... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>
newton.go:7:6: TS-S02: tiger can't tell how many times this loop runs: its condition is not a constant, a len, or a counter — add a cap that fails when the cap is hit, or make it an event loop that selects on ctx.Done() — see `tiger pattern --rule TS-S02`
status.go:11:2: TS-S08: this switch over Status has no default arm — add default: assert.Unreachable("Status: unhandled value") so a new Status constant fails loudly
tree.go:7:6: TS-S01: chase → chase is a cycle of calls (recursion) — replace it with a capped worklist loop that returns an error when it stops at max with nodes left — see `tiger pattern capped-worklist`
worker.go:6:2: TS-C02: this go statement starts a goroutine nobody owns — start it with wg.Go or errgroup.Go and Wait for the group — see `tiger pattern --rule TS-C02`
worker.go:6:2: TS-C03: this package starts goroutines but its tests have no goleak check — add a TestMain that calls goleak.VerifyTestMain(m)
tiger: 6 blocking
```

Tiger enforces the restrictions NASA's [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf)
and TigerBeetle's
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) put on
flight and database software. Whole classes of
production bugs can't be written, code tests in isolation, and your review goes to what the
change does instead of how it's built.

### Add to your agent instructions file

```markdown
- Run `tiger check ./...` before you finish. A finding fails the run, and there are no warnings.
- Fix a finding by rewriting the code to follow the rule. Tiger ignores `//nolint`.
- A finding that ends in "see `tiger pattern ...`" points to a pattern. Run that command and
  copy the shape it prints.
```

## Why the rules exist

Each rule stops one shape of bug. The left of each picture is code tiger reports, and the right
is the rewrite that passes. The pictures show the changed lines, not whole files. Dropped into a
file as drawn, the rewrites also draw two TS-S21 findings (a new limit constant needs a
compile-time relation to another quantity) and one TS-C03 finding (a package that starts
goroutines needs a `goleak` check in its tests). Adding a relation for each limit constant (for
example `const _ = uint(1<<31 - 1 - nodesMax)`) and a `TestMain` that calls
`goleak.VerifyTestMain(m)` in a test file makes `tiger check` print nothing. The rules are in the
[Tiger Rule Reference](docs/Tiger%20Rule%20Reference.md).

### TS-S01: no recursion

**From:** both sources. Rule 1 of NASA's [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) bans
"direct or indirect recursion", and the
[Safety section of TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety) says
"Do not use recursion".

A recursive function's stack depth follows its input, and the input comes from outside. Go lets
one stack grow to 1 GB before it gives up, so a deep input does not fail fast. The process quietly
eats a gigabyte and dies far from the cause.

![TS-S01 recursion: before, chase calls itself, so stack depth follows the input and a deep input grows the stack toward 1 GB; after, a loop visits stack[i] and appends its children, counts i up to nodesMax and returns errTooManyNodes if nodes are left](docs/recursion.svg)

### TS-S02: every loop has an upper bound

**From:** both sources. Rule 2 of [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) says
"All loops must have a fixed upper-bound", and the
[Safety section of TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety) says
"all loops and all queues must have a fixed upper bound".

A loop with no stated limit assumes its input is always well-formed. When it is not, the loop
hangs and reports nothing. In trials on two real codebases, two git tags pointing at each other spun a git server's
worker goroutine at full speed forever, a denial of service anyone could trigger remotely.

![TS-S02 loop bound: before, a for loop runs until converged returns true, so if refine stalls it never ends and reports nothing; after, the loop counts tries up to triesMax and returns errNoConvergence when it runs out](docs/loop-bound.svg)

### TS-C02: every goroutine starts through a group that waits for it

**From:** Tiger. Neither [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) nor
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) has a rule about starting
threads or waiting for them. TigerBeetle runs on a single-threaded event loop, while Go starts a
goroutine with a two-letter keyword, so Tiger adds the rule.

A goroutine that nothing waits for keeps running after its caller moves on. A server's memory
climbs for no clear reason, and a shutdown can finish while the goroutine is still working. In trials, a daemon's `WaitGroup` went
unwaited, so its `Shutdown` returned before its HTTP goroutines had exited.

![TS-C02 goroutine group: before, Launch starts a goroutine with a bare go statement and returns, so nothing waits for it and it can outlive its caller; after, Start launches it with wg.Go and Shutdown closes stop and calls wg.Wait on the same group until it exits](docs/goroutine-group.svg)

### TS-S08: a switch over a closed set ends in assert.Unreachable

**From:** Tiger. Neither [Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) nor
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) requires a switch to
handle every value of its type. Go has no enum type, so its compiler cannot check it either, and Tiger
adds the rule. The crash in the default arm follows
TigerStyle's Safety section, which says "The only correct way to handle corrupt code is to crash."

Adding a value to a type should break every switch that has not handled it. Without a default arm,
the new value falls out of the switch and the code carries on with a zero value and no error. In
trials, two storage backends silently dropped an action this way, and a git pack writer wrote an
invalid type code for a value it did not expect.

![TS-S08 closed switch: before, a switch over Status handles Active and Inactive with no default arm, so a new StatusArchived falls out and returns an empty string with no error; after, golangci-lint's exhaustive check reports the missing StatusArchived case, and a default arm that calls assert.Unreachable crashes if the value still arrives at run time](docs/closed-switch.svg)

## At a glance

**What tiger enforces**

- Loops that provably end, or stop at a cap that reports when it is hit
- Goroutines started through a group that waits for them, and blocking waits that have a stop case (a
  `select` case that ends the wait on cancellation or shutdown)
- Switches over a closed set that handle every value, with golangci-lint's `exhaustive` reporting a
  missing case and a default arm ending in `assert.Unreachable` as the backstop
- No direct calls to the clock, through golangci-lint
- With a layer file, randomness and IO packages only where the file allows them
- Named invariants, asserted in code and violated on purpose in a test
- A checked-in import graph that says which package may use which
- Size and complexity limits, through golangci-lint's own linters

**Where it runs**

- [`tiger check`](docs/Tiger%20User%20Guide.md#commands), the pass-or-fail gate in CI
- [`tiger changed`](docs/Tiger%20User%20Guide.md#posting-the-report-on-a-pull-request), a report on each pull request
- [`tiger changed-tests`](docs/Tiger%20User%20Guide.md#tiger-changed-tests), the existing tests a branch modifies or deletes
- [The golangci-lint plugin](docs/Tiger%20User%20Guide.md#running-under-golangci-lint), for your editor

## What you get

### Agents can't argue their way past it

`tiger check` passes or fails. It has no warnings and never honors `//nolint`. Its only escapes
are two directives that must carry a reason and count against a per-package budget. Under golangci-lint, the plugin included, `//nolint` works as it
does for any linter, which is why CI gates on `tiger check`. An agent heading for a green run
takes the cheapest path there, and with tiger the cheapest path is rewriting the code until it
follows the rule.

### It finds real bugs

In trials on two real codebases, tiger's checks found a git server that allocated 352 MB for a
12-byte push, list endpoints that accepted any page size up to 2^31−1, storage backends that
silently dropped an action because a switch had no default case, queue waits that could never be
cancelled, and a daemon whose `Shutdown` returned before its HTTP goroutines had exited.

### Code you can test in isolation

The golangci-lint config tiger generates bans every direct read of the clock (`time.Now`,
`time.Since`, `time.Until`, `time.After`, `time.Tick`, `time.NewTimer`, `time.NewTicker`,
`time.AfterFunc`) and `time.Sleep`, and with a layer file a
package imports randomness and IO packages only when its entry names them. Code built that way
takes its clock, random source and store as inputs. A test that controls them gets the same run
every time, so failures reproduce, timing tests stop flaking, and an agent debugging a
production issue works from a rerun instead of a guess.

### Reviewers see the architecture change first

Run `tiger changed main` and tiger lists, for each function, the calls into other packages it
started or stopped making since `main`. When an agent adds a database call inside an HTTP
handler, the handler shows up in the report with the new call, and the reviewer knows before
opening the diff. In CI the report is one pull request comment that updates on every push.

### The right shape for what no rule can check

Some bugs live where no analyzer can see, such as whether every caller waiting on a loop also
watches for shutdown. For those, tiger ships patterns: small runnable packages showing the
correct shape beside the broken one, with tests proving both and a checklist for the reviewer.
A finding from a rule a pattern backs ends with a pointer to it, and `tiger pattern
shutdown-loop` prints one.

### The same verdict every time

Tiger is deterministic static analysis. No language model sits in the verdict and tiger calls no
outside service, so the same code gets the same answer on your laptop and in CI.

## Try it

```
go install github.com/kapetan-io/tiger/cmd/tiger@latest
tiger check ./...
tiger changed main
```

A first run on an existing codebase reports a lot. Tiger keeps no baseline of old findings. A
codebase adopts it by fixing what it reports, with two documented aids described in the
[User Guide](docs/Tiger%20User%20Guide.md#the-committed-files). `comments.allow` in `tiger.yaml`
exempts a directory from the ban on prose comments, and `tiger budget --write` records the
escapes and skipped tests a package already carries.

## Docs

- [Tiger Explainer](docs/Tiger%20Explainer.md) is the place to start: what tiger is for and why,
  its concepts, and the real bugs its rules caught in trials.
- [Tiger User Guide](docs/Tiger%20User%20Guide.md) covers every command, the files a repository
  commits for tiger, the pull request workflow and the golangci-lint plugin's settings.
- [Tiger Rule Reference](docs/Tiger%20Rule%20Reference.md) has one entry per rule: what it
  requires and why, an example that fires, and the compliant rewrite.
- [Tiger Specification](docs/Tiger%20Specification.md) is the full standard, every rule with its
  reasoning, that the analyzers are built to.
- [Assert Adoption Playbook](docs/Assert%20Adoption%20Playbook.md) is the sequence for adding an
  `assert` package to a codebase that has none, which TS-S18 and TS-S08 need and the
  invariant rules build on.
- [Tiger Source Layout](docs/Tiger%20Source%20Layout.md) maps each package of this repository,
  for anyone changing tiger itself.
- [docs/CONTEXT.md](docs/CONTEXT.md) defines the terms the code, tests and docs use.
- [docs/adr/](docs/adr/) holds the architecture decisions, one numbered file each.

## Development

Tiger checks itself. CI runs `go vet ./...`, `go test ./...`, golangci-lint and
`tiger check ./...` over this repository, and every pattern must pass `tiger check` with zero
findings.

```
go test ./...
go run ./cmd/tiger check ./...
```
