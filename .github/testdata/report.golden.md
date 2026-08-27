```
internal/queue/drain.go:41:2: TS-S02: tiger can't tell how many times this loop runs: its condition is not a constant, a len, or a counter — add a cap (for tries := 0; tries < max; tries++) that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
internal/queue/drain.go:88:3: TS-C05: this channel operation blocks outside a select, so shutdown can't interrupt it — wrap it in a select that also has case <-ctx.Done(): return ctx.Err()
```

**3 advisories** — these never fail the run.

| rule | count | where |
|---|---:|---|
| `TS-L09` | 2 | internal/queue/drain.go:12:2<br>internal/queue/pool.go:30:2 |
| `TS-D07` | 1 | internal/queue/drain_test.go:19:2 |

<details><summary>all 3 advisory lines</summary>

```
internal/queue/drain.go:12:2: TS-L09 [advisory]: //tiger:batched "cursor drain, bounded by the store's page size" waives a rule here; tiger cannot check the claim, so this notice stands on every run — remove the directive when the constraint no longer holds
internal/queue/pool.go:30:2: TS-L09 [advisory]: //tiger:batched "reader loop, bounded by the payload length prefix" waives a rule here; tiger cannot check the claim, so this notice stands on every run — remove the directive when the constraint no longer holds
internal/queue/drain_test.go:19:2: TS-D07 [advisory]: this test is skipped, so it passes without running — remove the Skip call when the test can run again; this notice stands until then
```

</details>

`tiger: 2 blocking, 3 advisory`
