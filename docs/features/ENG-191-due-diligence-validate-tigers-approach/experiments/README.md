# ENG-191 experiments

Eleven small Go modules back calls in `../decision.md`: six for the loop-bound
calls, `maporder` and `fullcoverage` for call 8, `newfromrev` for call 9,
`nolint` for call 10, and `changed` for call 16. Each holds
its own `go.mod`, so the repo's `go test ./...` and `tiger check ./...` skip
them. Each directory also commits the output of both commands, captured with
tiger built from `main` at `5126797`.

Run any one from its directory:

```sh
go test -count=1 -v ./...   # saved as test.out
tiger check ./...           # saved as tiger.out
```

## cursorbound

Asks whether restating a query's limit in the loop header bounds the loop
whatever the database does. `ListUncapped` is querator's shape today.
`ListCapped` is `for count := 0; count < limit && rows.Next(); count++`. The
test feeds both a fake cursor that stops early, agrees with the limit, or never
stops, and counts every `Next` call.

Tiger blocks only `uncapped.go` (TS-V01 and TS-S02) of those two.

`filtered.go` backs call 13's filtered-loop question: a list that skips rows
outside a namespace before it keeps one. `fakeTable` puts the matching rows at
the end of the table.

| Loop | Counts | Tiger | At runtime |
|---|---|---|---|
| `ListFilteredCountRows` | every row read, against `limit` | passes | empty page while 5 matches exist |
| `ListFilteredCountMatches` | only kept rows, in the body | TS-V01 and TS-S02 block | reads all 1,000,000 rows for one match |
| `ListFilteredTwoLimits` | kept rows against `limit` clamped to `pageMax`, rows read against `scanMax` | passes | fills a page; returns `ErrScanLimit` after 10,000 reads on a sparse table |

The two-limit loop first drew TS-S21 on `scanMax`. The relation that clears it,
`const _ = uint(scanMax - pageMax)`, says a scan must be able to fill a dense
page. Tiger can't see the short-page bug in `ListFilteredCountRows`: its only
other exit drains the cursor, so the counter reads as a page size.

## annotations

Asks what tiger actually checks when a loop carries `//tiger:variant` or
`//tiger:batched`. Each file holds one loop. The tests run each loop and report
whether it ends.

| File | Annotation | Tiger | Loop at runtime |
|---|---|---|---|
| `variant_honest.go` | `//tiger:variant len(pending)`, true | no finding | ends |
| `variant_lie_refill.go` | same, but the body appends | TS-V01 blocks | runs forever |
| `variant_lie_skip.go` | same, but `continue` skips the shrink | TS-V01 blocks | runs forever |
| `variant_cursor.go` | `//tiger:variant remaining` on `rows.Next()` | TS-V01 and TS-S02 block | not run |
| `batched_honest.go` | `//tiger:batched`, true reason | TS-V01 blocks | ends |
| `batched_lie.go` | same reason, endless cursor | identical to the honest one | runs forever |
| `batched_wrong_shape.go` | on `for !ready()` | TS-S02 and TS-V01 block | not run |
| `batched_no_reason.go` | no reason text | TS-L09 and TS-S02 block | not run |
| `restated.go` | none | no finding | ends after `limit` rows on an endless cursor |

The one TS-C02 finding in `tiger.out` is the test helper's goroutine. A loop
that never returns can't be joined, so the helper leaks it on purpose. The
same holds in `variants`.

## variants

Asks which `//tiger:variant` expressions other than `len(pending)` tiger can
verify. The tests run each loop and report whether it ends.

| Loop | Variant | TS-V01 | TS-S02 | Loop at runtime |
|---|---|---|---|---|
| `Countdown` | `i` | verified | passes | ends |
| `PopAll` | `len(stack)` | verified | passes | ends |
| `EveryOther` | `n - i` | verified | blocks | ends |
| `Reverse` | `high - low` | verified | blocks | ends |
| `DrainQueue` | `len(q.Items)` | rejected, field in the condition | passes | ends |
| `WrongWay` | `n - i` with `i--` | rejected, ranking grows | blocks | runs forever |
| `Varint` | `size` with `size >>= 7` | rejected, shift not a known step | passes | ends |
| `Chunks` | `len(data)` with `data[len(chunk):]` | rejected, step not a fixed number | passes | ends |

TS-S02 blocks two loops whose variant TS-V01 verified, because `i < n` and
`low < high` carry no counter in the loop header.

## gaps

Holds loops where tiger's verdict and the loop's runtime behavior disagree.
Each gap found while working on call 2 gets a loop here and a test that shows
what the loop really does. "Runs forever" means the loop is still running when
the test stops waiting after 300 ms.

| Loop | What it does wrong | Tiger today | Loop at runtime |
|---|---|---|---|
| `Walk` | TS-S01's worklist rewrite; counter chases a growing slice | no finding | runs forever on a cycle |
| `ShadowSlice` | shrinks a shadowing copy, not the loop's slice | no finding | runs forever |
| `AliasAppend` | grows the slice through a pointer taken before the loop | no finding | runs forever |
| `ClosureGrow` | grows the slice through a closure made before the loop | no finding | runs forever |
| `LabeledContinue` | `continue outer` from an inner loop skips the shrink | TS-S09 only (style) | runs forever |
| `Overflow` | `i <= math.MaxInt` wraps | TS-S02 only; TS-V01 verifies it | runs forever |
| `WrongCounter` | counter moves away from the limit | no finding | runs forever |
| `CounterUndone` | `i--` in the body cancels `i++` | no finding | runs forever |
| `CounterReset` | `i = 0` in the body | no finding | runs forever |
| `LimitGrows` | `n++` in the body | no finding | runs forever |
| `HelperGrow` | worklist grown by a helper, not `append` | no finding | runs forever |
| `MapGrow` | limit is `len(m)` and the body adds entries | no finding | runs forever |
| `OrCounter` | `i < 10 \|\| !done()` | no finding | runs forever |
| `FieldNamedLikeCounter` | condition reads field `r.i`, counter is local `i` | no finding | runs forever |
| `StepsPastLimit` | `i != 7` stepping by 2 | no finding | runs forever |
| `ByteWraps` | `uint8` counter `<= 255` wraps | no finding | runs forever |
| `SpinCapped` | unprovable condition behind `i < math.MaxInt` | no finding | runs forever |
| `SpinLimited` | the same cap taken as a parameter; caller passes `math.MaxInt` | no finding | runs forever |
| `RangeNaturals` | ranges over an iterator that never ends | no finding | runs forever |
| `GotoLoop` | loops with `goto` | TS-S09 only (style) | runs forever |
| `WalkCapped` | call 2's proposed capped worklist | TS-S21 on the cap constant | stops at 101 nodes on a cycle with no error |
| `WalkRange` | ranges over a worklist it appends to | no finding | visits only the root |
| `WalkReported` | call 2's compliant worklist: capped, returns an error when the cap is hit | no finding | walks a tree; returns an error on a cycle |
| `RunTicker`, `RunFlat` | `select` on `ctx.Done()` | no finding | stops on cancel |
| `RunUntilCancelled` | `for ctx.Err() == nil` | TS-S02 and TS-V01 block | stops on cancel; runs forever on `context.Background()` |

### Which caps must report (call 2, change 5)

These loops back the rule that decides when a cap must report and where its
limit may come from. "Under call 2" is the verdict the ratified rules would
give; tiger today reports nothing on any of them.

| Loop and caller | Under call 2 | Loop at runtime |
|---|---|---|
| `ReadLines(scanner, 2)`, other exit drains a `bufio.Scanner` | passes; a page size needs no report | returns 2 of 5 lines |
| `SpinReported(never, 1000)`, reports when hit | passes | returns `spin reached its limit` |
| `SpinFromConfig`, config clamped to `spinsMax` | passes | returns the error after 10,000 spins |
| `SpinBreak(never, math.MaxInt)`, internal `break` | blocks until it reports, and the call is flagged | runs forever |
| `Spinner{Max: math.MaxInt}.Spin`, limit in a field | blocks until it reports | runs forever |
| `SpinLimited(never, math.MaxInt)` | blocks until it reports, and the call is flagged | runs forever |
| `SpinReported(never, math.MaxInt)` | call flagged for passing the type's maximum | runs forever |
| `SpinFromConfigRaw`, unclamped config value | blocks until the value is clamped | runs forever |
| `SpinReported(never, 1 << 40)` | **known miss**: a large constant below the threshold | runs forever |
| `ScanForever(1 << 40)`, `bufio.Scanner` over an endless reader | **known miss**: counts as an outside stream | runs forever |

### Where a limit's bound must sit (call 2, change 6)

A loop limit gets its upper bound where the value enters the program. Tiger
traces the limit backward from the loop, so a binary decode counts as an entry
just like config or a request.

| Loop and caller | Under call 2 | Loop at runtime |
|---|---|---|
| `ListClamped`, request limit clamped to `listMax` | passes | returns 1,000 of 5,000 rows |
| `ListRaw`, request limit written into `ListOptions` as is (ENG-194) | flagged where the field is written | returns all 5,000 rows |
| `ParsePack`, header count checked against the pack size and `objectsMax` | passes | rejects a 12-byte pack claiming 4 million objects |
| `ParsePackRaw`, header count used as is (ENG-193) | flagged where the count is decoded | allocates room for 4 million entries |
| `CreateClamped`, partitions clamped in a helper | passes; the helper's result carries the bound | stops at 256 |
| `CreateRaw`, partitions reached through a returned slice (ENG-195) | flagged | runs forever |
| `SpinThroughValue`, the loop called through a function value | **known miss** | runs forever |

## limitfacts

A prototype of call 2's change 6, kept as the starting point for building it
into tiger. It is a `go/analysis` analyzer that exports a fact on every
parameter and struct field that bounds a counter loop, and flags any value
written into one that is not a constant, a length, another bounded limit, or
clamped against a declared maximum. Its `analysistest` run covers a config
value clamped and raw, a request limit clamped and raw through an interface, a
forwarded parameter, a computed `len(x)*2`, a pack header count with and
without a guard, and the deliberate misses (a clamp in another function, a call
through a function value, a limit reached through a slice).

`cmd/limitfacts` runs it standalone: `go run ./cmd/limitfacts ./...`. On the
trial pins it reported 9 querator sites (6 real, filed as ENG-194, and 3 where
the clamp sits in another function) and 1 git-server site (ENG-193).

What production needs that the prototype lacks:

- The bounded-result refinement: a function that returns a clamped value
  carries the bound to its callers. That clears the clamp-in-a-helper false
  findings and catches limits reached through a returned slice (ENG-195).
- Clamp matching by object identity; the prototype compares expressions as
  text.
- The call-site flag for `math.MaxInt`, the type's maximum, or a constant at
  least half of it.

## call7

The shapes behind call 7, each with a runtime test. "Tiger today" is
`tiger.out`; "Under call 7" is what the decided rules would report.

| Shape | Tiger today | Under call 7 | At runtime |
|---|---|---|---|
| `DrainLatch`, select on a `Done()` that returns nil | no finding | TS-S03/C05: not a `context.Context` | never stops |
| `Worker.Run`, a `shutdown` channel nothing closes | no finding | TS-S03/C05: nothing closes or sends on it | never stops |
| `UseWithEmptyReset`, an empty `Reset` before `Put` | no finding | TS-M05 | the next user sees `alice` |
| `UseWithZero`, `*b = Buf{}` before `Put` | **TS-M05 (false finding, the poolzero bug)** | passes | never leaks |
| `UnwaitedDaemon`, `Add`/`go`/`Done` with no `Wait` (querator's trial bug) | TS-C02 | TS-C02 | `Shutdown` returns while the goroutine runs |
| `WaitedDaemon`, `wg.Go` with `Wait` in `Shutdown` | no finding | passes | waits for the goroutine |
| `GoNoWaitDaemon`, `wg.Go` with no `Wait` | **no finding** | TS-C02: `Wait` never reached | `Shutdown` returns while the goroutine runs |
| `EarlyExitLoop`, a request case that returns on a flag (ENG-160's shape) | no TS-S03 finding | one-exit rule | cleanup skipped; `Shutdown` waits out its deadline |
| `StopDoneLoop`, close `stop` once, one exit, close `done` | no finding on the loop | passes | cleans up every time, survives an expired `Shutdown`, two concurrent `Shutdown`s both wait |

Tiger's other findings here are TS-C05 on the demo `Submit` sends and
`<-ready`, TS-M05 on `PeekPooled`'s read-only `Put`, and TS-C12 asking for the
channel types to share one file; none bear on call 7. The tests pass 50 of 50
runs under `-race`.

## maporder

Backs call 8. Shows the collect-then-sort shape call 8 accepts, and three map
loops whose output varies while tiger passes them or would pass them under a
looser call 8. Each test calls the function 200 times and counts the different
results it prints.

| Function | Shape | TS-T02 today | Different results in 200 calls |
|---|---|---|---|
| `SortedIDs` | append keys, then `sort.Strings` | blocks (call 8 makes it pass) | 1 |
| `UnsortedIDs` | append keys, no sort | blocks | more than 1 |
| `FirstThree` | map write gated by a counter carried across iterations | no finding | more than 1 |
| `RankByValue` | `slices.SortedFunc(maps.Keys(m), ...)` with a comparator that ties | no finding, the range target is not a map | more than 1 |
| `SortedValues` | append floats, then `sort.Float64s`; -0 and +0 tie | blocks; accepting `sort.Float64s` would pass it | more than 1 |

## fullcoverage

A prototype of TS-T02 with call 8's four changes, kept as the starting point
for building it into tiger: collect-then-sort with plain sorts, comparator
sorts accepted only when they compare every field of the element, a check on
every `maps.Keys`/`Values`/`All` result, and no reads of state an earlier
visit wrote. `evidence.md` holds the module-cache counts and every
classification.

`cases/` holds 72 functions: 9 real sites with one function per rewrite, the
`maporder` gaps, and 20 boundary probes built to tie. `TestSoundness` joins
the prototype's live verdict with 200 calls per function and fails if any
function it passes prints more than one result. The one documented exception
is `boundary.Invert` (last writer wins), a known miss. Every boundary in the
design is rejected, and each rejected probe really does vary: one array byte,
one field, `String()`, pointer, interface, float, bool and `time.Time` fields,
and asymmetric or calling tie-breaks.

On the module cache (678 of 970 modules loaded), plain sorts accepted 91 sites
and full coverage 4, all in tiger and none able to tie. Of 46 rejected
comparator sites, 22% are real leaks and 72% are total by an invariant the
check can't see, mostly `sort.Slice`. On the trial pins TS-T02 drops from 11
to 7 findings on querator and from 17 to 15 on git-server, with none added.

`cmd/fullcoverage` runs it standalone: `go run ./cmd/fullcoverage ./...`.
`cmd/scan` reproduces the module-cache scan.

What production needs that the prototype lacks:

- A decision on the extra allowlist tightening it carries: refusing
  `keys[i] = k; i++` and carried-state reads anywhere in the body. Across the
  module cache that finds 31 real leaks tiger misses and adds 47 false
  findings.
- Accepting comparators held in a variable, `sort.Slice` index comparators
  and `sort.Sort` with a `Less` method, each a 20 to 60 line extension.
- The known misses: last writer wins, a map-write value that calls a
  function, `golang.org/x/exp/maps.Keys` and `reflect.MapKeys`.

## newfromrev

Backs call 9. Asks whether golangci-lint's `--new-from-rev`, which tiger's
`config/golangci.yml` tells adopters to use, hides tiger's own findings. Each
test builds a git repository with two commits, `old.go` on the main line and
`new.go` on the branch, each with the same TS-S09 violation, then runs
golangci-lint built with the tiger plugin and `tiger check`.

| Run | Findings | Exit |
|---|---|---|
| plugin, no flag | `old.go` and `new.go` | 1 |
| plugin, `--new-from-rev=HEAD~1` | `new.go` only; nothing says `old.go` was dropped | 1 |
| plugin, `--new-from-rev=HEAD` | none | 0 |
| plugin, `--new-from-rev=HEAD~1`, run through a symlinked path | none, not even `new.go` | 0 |
| `tiger check` | `old.go` and `new.go` | 1 |

The tests need both binaries. Build the plugin with `golangci-lint custom`
from the repo root (v2.11.4) and tiger with `go build ./cmd/tiger`, then:

```sh
TIGER_GCL=<path>/tiger-gcl TIGER=<path>/tiger go test -count=1 -v ./...
```

`tiger.out` holds three findings in the test helpers, not in the fixture.

## nolint

Backs call 10. Asks what `//nolint` does to tiger's findings under
`tiger check` and under the golangci-lint plugin, and whether an opt-in "no
`//nolint`" finding reported at the file's `package` line could survive the
plugin. Each fixture under `testdata/` is copied into a fresh module.

| Fixture | `tiger check` | plugin |
|---|---|---|
| `dropped`: `_ = os.Remove(path)` | TS-E02, exit 1 | not run |
| `droppednolint`: the same with a bare `//nolint` | passes, the comment counts as TS-E02's reason | not run |
| `line`: `//nolint:tiger` on a TS-S09 line | TS-S09, exit 1 | passes |
| `filewide`: `//nolint:tiger` in the package doc comment | TS-P02 at the `package` line and TS-S09 | passes, both dropped |
| `excluded`: an `exclusions` rule in `.golangci.yml`, no comment | TS-S09, exit 1 | passes |

So an opt-in "no `//nolint`" check can be enforced under `tiger check` only.
Under the plugin a file-wide `//nolint:tiger` drops a finding at the
`package` line, and an `exclusions` rule drops any finding without touching the
code. The tests need both binaries, built as in `newfromrev`:

```sh
TIGER_GCL=<path>/tiger-gcl TIGER=<path>/tiger go test -count=1 -v ./...
```

`tiger.out` holds three findings in the test helpers, not in the fixtures.

## changed

Backs call 16. A prototype of `tiger changed <base>`: for each struct, or free
function, it lists the types and functions it calls now that it did not call
at the base revision. Nothing propagates to callers, and a new method on a type
the struct already calls is not reported. `evidence.md` holds the real-code
runs.

The tests build a fixture git repository from `changed/testdata`, with
`fakedb` as a stand-in database driver outside the module. They cover a
handler that starts calling the database, a change reported on the struct that
made it and not on its callers, an interface call named as the interface, pure
helpers staying silent, a removed call, refactors within a struct, a new
method on a known type staying silent, a new constructor being reported, and
the receiving end of a channel being reported. The known blind spots each have
a test too: a write through `io.Writer`, and a send over a channel.

On 10 querator commits it printed 5 lines, 2 of them real (aeb014c, where the
daemon starts constructing the Postgres stores). On git-server's head commit it
printed 173 lines, 110 from generated code; the mixed filter and skipping
generated files bring that to 36.

```sh
go run ./cmd/changed -base origin/main <dir>   # or: changed <base-dir> <head-dir>
go test -race -count=1 -v ./...
```
