# goroutine-owner

The type that starts a goroutine owns it: it keeps a WaitGroup, signals the goroutine to stop on
shutdown, and waits for it before Shutdown returns.

## Rules

TS-C02

## Gap

Tiger checks that a goroutine has an owner and a way to be stopped (TS-C02). It cannot check that
the owner waits for the goroutine on its shutdown path. `DaemonNoWait` starts its goroutine with
`wg.Go`, which TS-C02 does not inspect, and its `Shutdown` closes the stop channel and returns, so
tiger stays silent while the goroutine is still winding down.

## Bug

Querator's unwaited daemon (ENG-148): `Shutdown` returned while the server goroutine was
still running. `TestDaemonNoWaitReturnsEarly` shows `Finished` is false right after
`DaemonNoWait.Shutdown`, and `TestDaemonShutdownWaits` shows `Daemon.Shutdown` returning only once
`Finished` is true.

## Checklist

- Does Shutdown wait for every goroutine the type started, including ones started with wg.Go?
- Does the wait come after the stop signal, so it cannot block forever?
- Is there a test showing the goroutine has finished when Shutdown returns?
- Does every Shutdown path that returns, including error paths, pass through the wait?
