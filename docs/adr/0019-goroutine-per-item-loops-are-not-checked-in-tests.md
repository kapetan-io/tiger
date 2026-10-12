# 19. Goroutine-per-item loops are not checked in tests

Date: 2026-10-12

## Status

Accepted

## Context

TS-C09 fires on a loop that starts one goroutine per item, because in production that lets outside
traffic set the concurrency level. Its fix is a bounded queue drained by one worker.

ENG-191 call 6 found the rule firing only in tests
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, call 6). Every one of
querator's 12 findings is the fan-out-then-wait idiom a concurrency test uses on purpose:

```go
var wg sync.WaitGroup
wg.Add(len(requests))

for i := range requests {
	go func(idx int) {
		defer wg.Done()
		if err := c.QueueLease(ctx, requests[idx], responses[idx]); err != nil {
			...
		}
	}(i)
}
```

```
service/common_test.go:626:3: TS-C09: this loop starts a goroutine per item, so the goroutines react to each item instead of working at their own pace — append the items to a bounded queue and drain it from one supervisor goroutine
```

The test exists to put N concurrent requests on the server at once. A single-worker queue would
serialize them and defeat the test, so the rule's fix breaks the code it fires on.

## Decision

TS-C09 skips `_test.go` files. Production fan-out still fires.

## Consequences

- querator's 12 findings go away. Neither codebase had a production hit, so production behavior is
  unchanged.
- A test can start as many goroutines as it wants. A leak in one is left to TS-C03's goleak harness
  (ADR-0020), not to this rule.
- The exempted shape gets its own analyzer test case (ENG-191 follow-up 4).
