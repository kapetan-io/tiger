# shutdown-loop

A loop goroutine stops through one stop channel, runs its cleanup before it closes done, and every
caller that waits on the loop also selects on done. Shutdown's context bounds only the caller's wait.

## Rules

TS-C02, TS-S03, TS-C05, TS-S23

## Gap

Tiger checks how the loop starts and stops, and that channel operations sit inside a select. It
flags the early exit, but only as an advisory TS-S23 count, so `EarlyExitLoop` is recorded in
`tiger.budget.yaml` and the run stays clean. It cannot check that cleanup runs before `done` closes.
`EarlyExitLoop` returns from its request case when `inShutdown` is set, so `close(done)` runs and
the cleanup in the stop case never does. The goroutine is owned, waited on and select-based, so
nothing blocking fires. `Loop` has one exit, `serve`
returns only from the stop case, and `run` sets `cleaned` before `close(l.done)`. Tiger also cannot
see whether `Call` selects on `done` while it waits for the reply.

`Loop` is the right shape: its only exit is the stop case, cleanup runs before `done` closes, and
the context bounds only the caller's wait. Cleanup comes before `close(l.done)` because a caller
released by `done` may read the cleaned state through `Cleaned`. `ErrClosed` is returned to callers
once the loop has finished. `Shutdown` bounds only the wait by its context, so a caller whose context
expired can call it again, and it reports an expired context before the wait so the result never
depends on whether the loop finished first. `reply` never blocks, because `Reply` has room for one
result and each request is answered once. In `call`, the buffer can accept work the loop never
reads, so `done` must end the wait for the reply. `done` can win the select against a reply sent just
before it closed, so `call` checks `request.Reply` once more before it returns `ErrClosed`.
`EarlyExitLoop.Shutdown` reports success once `done` closes, cleanup or not.

## Bug

Querator ENG-160 and ENG-196: a loop skipped cleanup on an early exit, Shutdown let the caller's
context bound the cleanup, and waiters on the loop's reply did not select on done.
`TestEarlyExitLoopSkipsCleanup` shows `EarlyExitLoop.Shutdown` returning nil while `Cleaned` is
false. `TestShutdownContextBoundsOnlyTheWait` and `TestCallAfterShutdownReturnsClosed` show `Loop`
finishing after an expired context and returning `ErrClosed` to a late caller.

## Checklist

- Does every caller waiting on the loop's reply also select on done?
- Does the loop's cleanup run before it closes done, on every way the loop can exit?
- Does Shutdown close stop before it waits?
- Does the context passed to Shutdown bound only the caller's wait, never the cleanup?
- Is there a test that calls Shutdown with an expired context and then calls it again?
