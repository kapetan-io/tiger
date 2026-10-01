# ENG-191 experiments

Four small Go modules back the loop-bound calls in `../decision.md`. Each holds
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

Tiger blocks only `uncapped.go` (TS-V01 and TS-S02).

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
