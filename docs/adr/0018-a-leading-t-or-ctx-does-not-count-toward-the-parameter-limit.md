# 18. A leading t or ctx does not count toward the parameter limit

Date: 2026-10-12

## Status

Accepted

## Context

TS-N07 has two halves. It fires when two adjacent parameters share a type, because Go has no named
arguments and a caller can swap them. It also fires when a function takes more than four
parameters.

On querator and git-server, most of the four-parameter half's
findings are test helpers like this one:

```go
func assertPartition(t *testing.T, ctx context.Context, c *querator.Client, name string, expected Partition) {
```

```
service/partition_test.go:503:1: TS-N07: this function takes 5 parameters — group the related ones into an options struct
```

The two leading parameters are Go convention, and nothing after them can be swapped with them. They
use up half the limit before the helper does anything. The suggested fix wraps `name` and
`expected` in an options struct nobody would mix up, and agents learn to thread `t` through a
struct field, which reads worse.

## Decision

TS-N07 keeps its four-parameter limit. A leading `testing.TB` (or `*testing.T` or `*testing.B`),
and a `context.Context` that comes first or right after it, do not count toward the limit.

The adjacent-same-type half is unchanged. That half carries the swap hazards, like the 67
adjacent-`string` findings on git-server.

## Consequences

- `assertPartition` counts 3 and passes. Across the two codebases, 9 of querator's 10 limit findings
  and 12 of git-server's 21 clear, 21 in all.
- The findings that remain are real, like an eight-parameter `Finalize` in querator's benchmarks and
  an eight-parameter diff helper in git-server.
- A `t` or `ctx` anywhere but the leading positions still counts, so the exemption can't be used to
  hide a long parameter list.
- The exempted shape gets its own analyzer test case.
