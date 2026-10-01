# ENG-191 experiments

Two small Go modules back the loop-bound calls in `../decision.md`. Each holds
its own `go.mod`, so the repo's `go test ./...` and `tiger check ./...` skip
them. Each directory also commits the output of both commands, captured with
tiger built from `main` at `5126797`.

Run either one from its directory:

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
| `batched_honest.go` | `//tiger:batched`, true reason | TS-V01 blocks, TS-L09 notice | ends |
| `batched_lie.go` | same reason, endless cursor | identical to the honest one | runs forever |
| `batched_wrong_shape.go` | on `for !ready()` | TS-S02 and TS-V01 block | not run |
| `batched_no_reason.go` | no reason text | TS-L09 and TS-S02 block | not run |
| `restated.go` | none | no finding | ends after `limit` rows on an endless cursor |

The one TS-C02 finding in `tiger.out` is the test helper's goroutine. A loop
that never returns can't be joined, so the helper leaks it on purpose.
