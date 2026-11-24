// A Wait counts only when it is reached on the success path: not after a
// return, not on one branch, not inside a literal nobody calls, and not from
// the goroutine the group is waiting for. A ctx.Done() case excuses nothing
// before the start, around the start, or in a function that never waits.
package fixture

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// unreachableWait has a Wait after a return.
type unreachableWait struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (s *unreachableWait) Start() {
	s.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Close returns before it waits.
func (s *unreachableWait) Close() {
	return
	s.wg.Wait()
}

// oneBranchWait waits only when asked to.
type oneBranchWait struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (v *oneBranchWait) Start() {
	v.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Close waits on one branch, and returns on the other without waiting.
func (v *oneBranchWait) Close(skip bool) {
	if skip {
		v.wg.Wait()
	}
}

func conditionalWait(n int) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if n > 0 {
		wg.Wait()
	}
}

func waitsOnItself() {
	var wg sync.WaitGroup
	wg.Go(func() { wg.Wait() }) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// literalNeverCalled builds a function that waits and drops it.
type literalNeverCalled struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (u *literalNeverCalled) Start() {
	u.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Stop never calls the literal.
func (u *literalNeverCalled) Stop() {
	wait := func() { u.wg.Wait() }
	_ = wait
}

// waitBeforeGo waits before the goroutine starts, so the Wait joins nothing.
func waitBeforeGo() {
	var wg sync.WaitGroup
	wg.Wait()
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// waitAsValue takes Wait as a value and never calls it.
func waitAsValue() {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	_ = wg.Wait
}

// expiredBeforeTryGo passes a ctx.Done() case before TryGo starts anything.
// The case is not a Shutdown giving up on a started goroutine, so it does not
// stand in for the Wait.
func expiredBeforeTryGo(ctx context.Context) {
	var g errgroup.Group
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	default:
	}
	g.TryGo(func() error { return nil }) // want `TS-C02: this TryGo call starts a goroutine, but no Wait on the group is reached`
}

// expiredBeforeGo runs every path through a ctx.Done() case before the start.
func expiredBeforeGo(ctx context.Context) {
	var wg sync.WaitGroup
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// expiredBeforeFieldGo starts on a field group after every path runs through a
// ctx.Done() case, and Shutdown only signals.
type expiredBeforeFieldGo struct {
	wg   sync.WaitGroup
	stop chan struct{}
}

// Start passes the expired case and then starts.
func (e *expiredBeforeFieldGo) Start(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
	e.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Shutdown signals the goroutine and returns without waiting.
func (e *expiredBeforeFieldGo) Shutdown() { close(e.stop) }

// expiredBeforeFieldTryGo is the TryGo form on a field group: the expired case
// runs before TryGo, and no method waits.
type expiredBeforeFieldTryGo struct {
	g errgroup.Group
}

// Start passes the expired case or the default, then tries to start.
func (e *expiredBeforeFieldTryGo) Start(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	default:
	}
	e.g.TryGo(func() error { return nil }) // want `TS-C02: this TryGo call starts a goroutine, but no Wait on the group is reached`
}

// expiredFallsThrough passes a ctx.Done() case after the start in a function
// that never waits.
func expiredFallsThrough(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}

// expiredAfterTryGo gives up through a ctx.Done() case after TryGo and never
// waits on the other path.
func expiredAfterTryGo(ctx context.Context) error {
	var g errgroup.Group
	g.TryGo(func() error { return nil }) // want `TS-C02: this TryGo call starts a goroutine, but no Wait on the group is reached`
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

// startInExpired starts inside the ctx.Done() case it returns from; nothing
// started before the case, so the return is no Shutdown giving up.
func startInExpired(ctx context.Context) error {
	var wg sync.WaitGroup
	select {
	case <-ctx.Done():
		wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		return ctx.Err()
	}
}

// expiredOnlyIdle has an exported method that waits on nothing and only
// returns through a ctx.Done() case; it is not a Shutdown for the group.
type expiredOnlyIdle struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (o *expiredOnlyIdle) Start() {
	o.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Idle blocks until ctx expires.
func (o *expiredOnlyIdle) Idle(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}

// expiredNoWaitAnywhere returns through a ctx.Done() case after the start, but
// the function never waits, so it is no Shutdown giving up.
func expiredNoWaitAnywhere(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}

// expiredNoWaitTryGo is the TryGo form of expiredNoWaitAnywhere.
func expiredNoWaitTryGo(ctx context.Context) error {
	var g errgroup.Group
	g.TryGo(func() error { return nil }) // want `TS-C02: this TryGo call starts a goroutine, but no Wait on the group is reached`
	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}

// expiredStartNoWait starts on a field group and returns through a ctx.Done()
// case in the same method; no method waits.
type expiredStartNoWait struct {
	wg sync.WaitGroup
}

// Start runs the worker and blocks until ctx expires.
func (e *expiredStartNoWait) Start(ctx context.Context) error {
	e.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}

// startInNestedExpired starts inside a ctx.Done() case and returns from a
// ctx.Done() case nested in it; the start is still inside the giving-up case.
func startInNestedExpired(ctx, inner context.Context, done chan struct{}) error {
	var wg sync.WaitGroup
	select {
	case <-done:
		wg.Wait()
		return nil
	case <-ctx.Done():
		wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		select {
		case <-inner.Done():
			return inner.Err()
		}
	}
}
