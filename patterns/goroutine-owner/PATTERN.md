# goroutine-owner

The type that starts a goroutine owns it: it keeps a WaitGroup, signals the goroutine to stop on
shutdown, and waits for it on every way out of Shutdown, error paths included.

## Rules

TS-C02

## Gap

Tiger checks that a goroutine starts through a group and that a `Wait` on the group is reached in an
exported method of the owning type (TS-C02). It cannot check that every path through that method
reaches the `Wait`. `DaemonSkipsWait.Shutdown` closes the stop channel, flushes, and waits only when
the flush worked, so on a flush error it returns while the goroutine is still winding down. Tiger
stays silent because a `Wait` is there. `Daemon.Shutdown` waits first and flushes after.

## Bug

Querator's unwaited daemon (ENG-148): `Shutdown` returned while the server goroutine was still
running. TS-C02 now catches a daemon with no `Wait` at all, and this is the same bug on an error
path. `TestDaemonSkipsWaitReturnsEarly` shows `Finished` is false right after a failed
`DaemonSkipsWait.Shutdown`, and `TestDaemonShutdownWaits` shows `Daemon.Shutdown` returning only once
`Finished` is true, whether the flush works or fails.

## Checklist

- Does every path out of Shutdown, including error returns, pass through the wait?
- Does the wait come after the stop signal, so it cannot block forever?
- Is there a test showing the goroutine has finished when Shutdown returns, on the error path too?
- Does the type start every goroutine through the group it waits on?
