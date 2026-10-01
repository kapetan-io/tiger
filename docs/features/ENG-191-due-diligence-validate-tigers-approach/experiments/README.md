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
what the loop really does.

| Loop | Tiger | Loop at runtime |
|---|---|---|
| `Walk`, TS-S01's worklist rewrite | no finding | ends on a tree, runs forever on a cycle |
| `RunTicker`, `select` on `ctx.Done()` and a ticker | no finding | stops on cancel |
| `RunFlat`, `select` on `ctx.Done()` with `default` | no finding | stops on cancel |
| `RunUntilCancelled`, `for ctx.Err() == nil` | TS-S02 and TS-V01 block | stops on cancel |
