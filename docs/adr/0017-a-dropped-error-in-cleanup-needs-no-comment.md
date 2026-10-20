# 17. A dropped error in cleanup needs no comment

Date: 2026-10-12

## Status

Accepted

Superseded in part by [ADR-0031](0031-comments-belong-in-tests-not-code.md): a comment no longer clears TS-E02.
The cleanup exemption stands.

## Context

TS-E02 fires on `_ = f()` and `x, _ := f()` when the discarded value is an error, unless a comment
on that line or the line above explains why.

ENG-191 call 4 counted its findings on querator and git-server
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, call 4). Cleanup is
36 of querator's 86 and 11 of git-server's 42:

```go
defer func() { _ = d.Shutdown(ctx) }()
t.Cleanup(func() { _ = client.Close(context.Background()) })
```

The fix is always the same comment, something like `// best-effort cleanup`. The rule checks only
that a comment exists, never what it says. A fixture shows `// x` satisfies it, and
`experiments/nolint` shows a bare `//nolint` does too. So the 47 comments document nothing and teach
agents that any comment clears the rule.

The rule's one real bug in the trials was elsewhere: a hash-format validation error discarded, which
would have produced corrupt object ids the day a second hash format shipped.

ADR-0006 allows tuning when the misfires form a bounded shape that can be named and excluded.
"Inside `defer` or `t.Cleanup`" is that kind of shape. ENG-179 had proposed a `//tiger:discard`
directive for these sites instead, which would replace one unchecked comment with another.

## Decision

A discarded error inside a `defer` statement's call, or inside a function literal passed to
`t.Cleanup`, is not a TS-E02 finding. Every other discard still fires, including non-deferred
cleanup like `_ = resp.Body.Close()`.

No `//tiger:discard` directive is added. ENG-179's item 2 is dropped.

## Consequences

- 47 findings go away across the two trials, and the boilerplate comments with them.
- The hash-validation shape, the one that found a real bug, still fires.
- `defer func() { _ = f.Close() }()` on a file opened for writing drops the write error, a real
  bug class, and tiger does not report it. It is not caught today either, since any comment clears
  the rule. Review has to catch it.
- The exemption ships with a test case for each exempted shape and for the non-deferred cleanup
  that still fires.
