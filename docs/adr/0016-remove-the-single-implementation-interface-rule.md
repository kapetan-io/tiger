# 16. Remove the single-implementation interface rule

Date: 2026-10-12

## Status

Accepted

## Context

TS-X01 fires on any interface with exactly one implementation outside `_test.go` files, on the
theory that such an interface is abstraction added just in case. Its message tells the developer
to delete the interface and use the concrete type.

Run on querator and git-server, the rule fired 24
times, and all 24 were deliberate seams: narrowing a capability, a test boundary, or a package
boundary. None was the speculative abstraction the rule targets.

The clearest case is git-server's `RefReader`. The ref store can read refs and also change them,
including through `CompareAndSwap`. The graph API only needs to read, so it receives a
`RefReader` with just `List` and `Resolve`. Graph-query code then cannot change refs, and invariant
I2 ("graph queries never change refs") is enforced by the Go compiler. Only the ref store
implements `RefReader`, so TS-X01 fires:

```
services/git-server/internal/graphapi/graphapi.go:32:6: TS-X01: interface RefReader has one implementation, memory.refStore — use memory.refStore directly and delete the interface, or add a second implementation outside _test.go files
```

Following that message puts `CompareAndSwap` in reach of every graph query. The other 14
git-server findings are storage ports a conformance suite tests, the event `Emitter` seam, and the
`Repos` and `RepoPolicy` seams between packages. querator's 9 are role interfaces like `QueueAdmin`
and `UsersAdmin` over one `service.Service`, where the fix widens every consumer's dependency to the
whole service.

"One implementation" looks the same whether an interface is pointless or is guarding an invariant.
ADR-0006 removes a rule whose output on real codebases is dominated by findings a reader would
decline to act on, including any finding whose named remedy would break the code. TS-X01 meets that
on both codebases.

## Decision

TS-X01 is removed end to end under ADR-0006: the analyzer, its test cases, its registry entry, and
the specification's enforcement line.

The maxim against speculative interfaces stays in the specification, with review as its
enforcement. Whether an interface is the right seam is a design judgment.

## Consequences

- The 24 findings go away, and no agent is pushed to delete an interface that narrows a capability.
- A truly speculative single-implementation interface is no longer reported. Review has to catch
  it.
- The reason is general: a shape rule cannot tell a speculative interface from one that narrows
  capability, and a remedy that deletes the interface damages the second kind. A future rule that
  judges interfaces by their implementation count meets the same objection.
- An advisory trial was the alternative. The git-server rerun answered the
  question the trial would have asked, so no trial runs.
