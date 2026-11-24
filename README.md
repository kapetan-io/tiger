# Tiger Go

A static analyzer that holds Go written by AI agents to the rules NASA wrote for flight software.

Let agents write the code and let tiger hold the line. Tiger enforces the restrictions NASA's
[Power of Ten](https://spinroot.com/gerard/pdf/P10.pdf) and TigerBeetle's
[TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md) put on
flight and database software. Every loop has a bound, every goroutine has an owner that waits
for it, and code takes the clock as an input instead of reading it. Whole classes of production
bugs can't be written, code tests in isolation, and your review goes to what the change does
instead of how it's built.

## At a glance

**What tiger enforces**

- Loops that provably end, or stop at a cap that reports when it is hit
- Goroutines started through a group that waits for them, and waits that can be cancelled
- Switches over an enum that handle every value
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

A first run on an existing codebase reports a lot. Tiger keeps no list of old findings to ignore,
so a codebase adopts it by fixing what it reports.

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
