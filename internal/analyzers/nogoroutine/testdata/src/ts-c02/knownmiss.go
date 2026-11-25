// Documented TS-C02 coverage gaps.
package fixture

import (
	"context"
	"sync"
)

func startOnParameter(wg *sync.WaitGroup) { wg.Go(work) }

// callerNeverWaits passes its group to a function that starts a goroutine on
// it and never waits.
//
// known-miss: a group received as a parameter is the caller's to wait on, and
// this check does not follow the argument back to see that the caller does
// not.
func callerNeverWaits() {
	var wg sync.WaitGroup
	startOnParameter(&wg)
}

// waitOnEarlyPath waits only on a path that returns before the start, then
// gives up through a ctx.Done() case after it.
//
// known-miss: any Wait on the group in the function lets a ctx.Done() case
// after the start stand in for one, even a Wait the path never reaches or
// reaches only before the start (waitBeforeGiveUp, expiredWaitBeforeGiveUp).
func waitOnEarlyPath(ctx context.Context, early bool) {
	var wg sync.WaitGroup
	if early {
		wg.Wait()
		return
	}
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}

// waitBeforeGiveUp waits before the start, then gives up through a ctx.Done()
// case after it. The known-miss note on waitOnEarlyPath covers it.
func waitBeforeGiveUp(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Wait()
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}

// expiredWaitBeforeGiveUp waits only in a ctx.Done() case before the start,
// then gives up through another after it. The known-miss note on
// waitOnEarlyPath covers it.
func expiredWaitBeforeGiveUp(ctx context.Context) {
	var wg sync.WaitGroup
	select {
	case <-ctx.Done():
		wg.Wait()
	default:
	}
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}

// branchWaitSentinel leaks the goroutine when Closed is called with false.
type branchWaitSentinel struct{ wg sync.WaitGroup }

// Start runs the worker.
//
// known-miss: Closed returns only a non-nil error, so it has no success path
// to check, and its Wait on one branch makes it count as a Wait on every path.
func (b *branchWaitSentinel) Start() { b.wg.Go(work) }

// Closed waits only when asked to, then reports the group closed.
func (b *branchWaitSentinel) Closed(wait bool) error {
	if wait {
		b.wg.Wait()
	}
	return errClosed
}

// deferForever never runs its deferred Wait, because Run never returns.
type deferForever struct{ wg sync.WaitGroup }

// Start runs the worker.
//
// known-miss: Run has no success path to check, and its deferred Wait makes it
// count as a Wait though the Wait never runs.
func (d *deferForever) Start() { d.wg.Go(work) }

// Run defers a Wait and loops forever.
func (d *deferForever) Run() {
	defer d.wg.Wait()
	for {
	}
}
