# ENG-177 trial rerun

Both trial codebases rerun with the ADR-0020 rules built. The baseline is tiger at the commit this
branch started from (`fde83d5`), the new run is this branch, and both ran `tiger check ./...` on the
same checkout.

| Codebase | Commit | Checkout |
|---|---|---|
| querator | `2824f64` (main, has the ENG-158 and ENG-160 fixes) | `~/Development/querator` |
| git-server | mono-repo main, generated code from `task proto-gen` and `task slipclient-gen` run in a scratch copy | `services/git-server` |

## Finding counts

| Rule | querator before | querator after | git-server before | git-server after |
|---|---|---|---|---|
| TS-C02 | 16 | 16 | 9 | 9 |
| TS-C03 | 0 | 3 | 0 | 2 |
| TS-C05 | 16 | 14 | 7 | 7 |
| TS-S03 | 0 | 1 | 0 | 0 |
| TS-S23 (advisory) | 0 | 2 | 0 | 0 |
| TS-M05 | 0 | 0 | 0 | 0 |

## TS-S23 advisory counts

querator has 2 findings, both in package `internal`, git-server has none.

- `internal/handlers.go:136` returns from the paused-queue loop when a pause request cancels the pause.
  The loop has a second way out on purpose, so a reader would decline to act on this one.
- `internal/logical.go:687` returns from the request case once `state.Shutdown` is set, the shape
  ENG-160 fixed. `state.Shutdown` is set only inside `handleShutdown` (`internal/handlers.go:260`), so
  the return is the fixed design, which ADR-0020 says this rule also flags.

One finding is actionable and one is not, so the trial leaves the rule at advisory.

## What moved

- **TS-C03** is new. querator's `daemon`, `docs` and `internal` packages and git-server's `conformance` and
  `internal` packages start goroutines with no `goleak` in a `TestMain`.
- **TS-C05 and TS-S03** gained `cmd/quickstart/main.go:194` (querator). Its context comes from
  `context.WithTimeout(context.Background(), ...)` in the same function, which ADR-0020 does not
  count as a stop case, and `docs/embedded.go:148`, a bare `<-ctx.Done()` on a context made from
  `Background` in the same function. In git-server `stats_test.go:43` is a bare send on a channel
  called `done`, which the name no longer exempts.
- **TS-C05** dropped 4 findings in querator's tests (`queue_test.go:1734`, `queue_test.go:3553`,
  `shutdown_test.go:268`, `shutdown_test.go:326`) and 1 in git-server's (`service_toggle_test.go:66`).
  Each selects on a result channel that a goroutine in the same package sends on, which ADR-0020
  accepts as a stop case.
- **TS-C02** is unchanged in both, with no new `wg.Go` or `errgroup.Go` call that lacks a `Wait`.

## Reading

A channel the package sends on counts as a stop case, so a result or request channel makes a
select pass. That is how ADR-0020 states the rule, and it is why TS-C05 lost findings in tests. It
also means TS-S23 cannot use the same definition to find the stop case, because the request channel
`Submit` sends on would then exempt the very return it should flag. TS-S23 treats a case on a channel
the package only sends on as the stop case only when it ends in a `return`.
