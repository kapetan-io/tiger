# 20. Concurrency rules check behavior, not names

Date: 2026-10-12

## Status

Accepted

## Context

Four blocking rules trust a name where they should check what the code does:

- TS-C02 requires every `go` statement to start through an owner that waits for it. The only
  recognized owner is `errgroup.Group.Go`.
- TS-S03 requires a loop that runs forever to select on a case that can stop it, and TS-C05 requires
  a blocking channel wait to have a case that can cancel it. Both accept `<-x.Done()` for any `x`
  with a method named `Done`, and any channel whose name contains `shutdown`, `stop`, `quit`, or
  `done`.
- TS-M05 requires a value put back in a `sync.Pool` to be reset first. It accepts any method named
  `Reset`, including an empty one.

A fixture written to test these rules passes `tiger check` with a `Done()` that never fires,
a `shutdown` channel nothing closes, and an empty `Reset`. Renaming them gets 5 blocking findings.
TS-C02 only looks at `go` statements, so `wg.Go(f)` or `errgroup.Go(f)` with no `Wait` anywhere
passes. That is querator's trial bug rewritten in a form tiger accepts: `daemon.go` calls `wg.Add`
and `wg.Done` but never `wg.Wait`, so `Shutdown` returns while its goroutines still run.

Three designs were argued with probes on querator and git-server. Recognizing behavior catches most
fakes and rejects none of querator's correct code. Accepting only exact shapes catches every fake
but rejects querator's request-carrying `shutdownCh` and the best shutdown design below. Enforcing
only at runtime with goleak missed querator's unwaited daemon in 50 of 50 runs, because the
goroutine exits a moment after `Shutdown`, before goleak looks.

Five shutdown designs were then built on querator's main and run against the ENG-160 race test and
five edge cases: the caller's ctx expiring while the loop is busy, a retry after that, two concurrent
`Shutdown` calls, a pause queued before shutdown, and a cold request queued before shutdown. Only the
`net/http` shape, with every waiter also selecting on `done`, passed all of them. Strict, exact-shape
TS-S03 rejects that design and accepts only the one that stores a cancellable context and keeps
the request in an atomic pointer, which fails the retry, double and cold-request cases. So exact shapes are right for TS-C02 and TS-M05 and wrong for TS-S03 and TS-C05.

Go's own guidance points the same way. `net/http`'s `Server.Shutdown` sets a flag and closes its
listeners first, lets the caller's ctx bound only the wait, and uses the flag only to classify why
the loop woke up. Uber's style guide stops a goroutine by closing a `stop` channel and waits on a
`done` channel, with the stop case as the loop's only `return`. Google's guide and the `context`
docs say not to store a context in a struct.

None of the static designs catch the four querator bugs the edge tests found: an abandoned shutdown
when the caller's ctx expires, a second `Shutdown` returning early, a panic on a queued cold request,
and a cold request never answered. Only running the shutdown paths does. They are filed as ENG-196.

## Decision

All four rules stay blocking. TS-C02 and TS-M05 accept only exact shapes. TS-S03 and TS-C05
recognize behavior.

| Rule | Accepts |
|---|---|
| TS-C02 | only `errgroup.Group.Go` and `sync.WaitGroup.Go`; a bare `go` statement is always a finding. The group's `Wait` must be reached on the success path: in the same function, or for a field, in an exported method of the owning type. A `Shutdown` that returns early on its own ctx expiring is exempt on that path |
| TS-S03, TS-C05 | `.Done()` only on a `context.Context`, and not on one that traces within the function to `context.Background()` or `context.TODO()` |
| TS-S03, TS-C05 | a channel the package closes or sends on, on the same object: for a struct field, in a method of the owning type reachable from an exported method |
| TS-M05 | only `*b = T{}` before `Put`. The poolzero bug that flags that form after `pool.Get` is fixed first |

A new rule requires an event loop to have one exit, its stop case. In a TS-S03 loop, no `return` or
break-out may sit outside the recognized stop case. It is a syntax check, so it is exact. It is
measured on querator and git-server before it blocks.

TS-C03, `goleak` in `TestMain` for every package that starts goroutines, is implemented as a shape
check. Tiger blocks a package with a `go`, `wg.Go` or `errgroup.Go` and no goleak harness.

The TS-S03 and TS-C02 messages name one shape as the fix: set a flag, close a stop channel once, let
the loop's only exit be its stop case, have the loop close `done`, start it with `wg.Go`, and have
every waiter select on `done`. TS-S03's message also points to shutdown edge tests like the five
above.

## Consequences

- Renaming a channel `done` or writing an empty `Reset` no longer passes three blocking rules, and
  `wg.Go` with no `Wait` no longer passes TS-C02.
- Correct shutdown code, including querator's best design, passes. The shape every message points to
  is the one Go's own `net/http` uses.
- The one-exit rule flags the shape behind ENG-160. It also flags the correct handshake design, which
  pushes agents toward closing a stop channel; its message names the `net/http` shape as the fix.
- TS-S03 and TS-C05 still miss a `close` in dead code and a `Wait` in a method nobody calls. The
  goleak harness and the shutdown edge tests are the runtime backstop, and goleak alone misses a
  goroutine that exits just after `Shutdown`, so the static checks stay.
- Whether every waiter also selects on `done` is beyond any of these rules. The shutdown-loop
  pattern covers it.
- This revives ENG-177 as revised. Each fixture above becomes
  an analyzer test case, and both codebases are rerun.
