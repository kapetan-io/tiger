// A function with no success exit counts as a Wait only when the Wait runs on
// every exit it does have, or, when it never returns, on every path into its
// endless loop, where a deferred Wait never runs.
package fixture

import "sync"

// branchWaitSentinel leaks the goroutine when Closed is called with false.
type branchWaitSentinel struct{ wg sync.WaitGroup }

// Start runs the worker.
func (b *branchWaitSentinel) Start() {
	b.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

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
func (d *deferForever) Start() {
	d.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run defers a Wait and loops forever.
func (d *deferForever) Run() {
	defer d.wg.Wait()
	for {
	}
}

// errorBeforeDefer returns an error before it reaches its deferred Wait.
type errorBeforeDefer struct{ wg sync.WaitGroup }

// Start runs the worker.
func (e *errorBeforeDefer) Start() {
	e.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed reports the flush error before it defers the Wait.
func (e *errorBeforeDefer) Closed(flush func() error) error {
	if err := flush(); err != nil {
		return err
	}
	defer e.wg.Wait()
	return errClosed
}

// deferLiteralForever defers a literal that waits, and never returns.
type deferLiteralForever struct{ wg sync.WaitGroup }

// Start runs the worker.
func (d *deferLiteralForever) Start() {
	d.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run defers the waiting literal and loops forever.
func (d *deferLiteralForever) Run() {
	defer func() { d.wg.Wait() }()
	for {
	}
}

// branchInLoop waits only on some passes of a loop that never ends.
type branchInLoop struct{ wg sync.WaitGroup }

// Start runs the worker.
func (b *branchInLoop) Start() {
	b.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run waits only when ready says so, forever.
func (b *branchInLoop) Run(ready func() bool) {
	for {
		if ready() {
			b.wg.Wait()
		}
	}
}
