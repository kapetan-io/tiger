```
internal/queue/drain.go:41:2: TS-S02: unbounded loop has no verified variant — restate the bound or annotate it
internal/queue/drain.go:88:3: TS-C05: blocking receive on ch has no ctx.Done() case
```

**3 advisories** — these never fail the run.

| rule | count | where |
|---|---:|---|
| `TS-L09` | 2 | internal/queue/drain.go:12:2<br>internal/queue/pool.go:30:2 |
| `TS-D07` | 1 | internal/queue/drain_test.go:19:2 |

<details><summary>all 3 advisory lines</summary>

```
internal/queue/drain.go:12:2: TS-L09 [advisory]: escape //tiger:batched — cursor drain, bounded by the store's page size
internal/queue/pool.go:30:2: TS-L09 [advisory]: escape //tiger:batched — reader loop, bounded by the payload length prefix
internal/queue/drain_test.go:19:2: TS-D07 [advisory]: skipped test — a skipped test is a test that passes; this notice stands until the Skip call is removed
```

</details>

`tiger: 2 blocking, 3 advisory`
