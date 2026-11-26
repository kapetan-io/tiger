// A function with no success exit counts as a Wait only when code that may
// wait runs before every return it has, or, when it never returns, when no
// path goes round a loop forever without it. A deferred Wait never runs there.
package fixture

import (
	"context"
	"sync"
)

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

// namedBranchWait returns its named error result, waiting on one return only.
type namedBranchWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (n *namedBranchWait) Start() {
	n.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed waits only when c is set, and returns the set error either way.
func (n *namedBranchWait) Closed(c bool) (err error) {
	err = errClosed
	if c {
		n.wg.Wait()
		return err
	}
	return err
}

// stopWithoutWait returns its stop error from a select loop without waiting.
type stopWithoutWait struct {
	wg   sync.WaitGroup
	stop chan struct{}
	tick chan struct{}
}

// Start runs the worker.
func (s *stopWithoutWait) Start() {
	s.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run waits on each tick, but returns on stop without waiting.
func (s *stopWithoutWait) Run() error {
	for {
		select {
		case <-s.stop:
			return errClosed
		case <-s.tick:
			s.wg.Wait()
		}
	}
}

// deferInEndlessLoop defers its Wait inside a loop that never ends.
type deferInEndlessLoop struct{ wg sync.WaitGroup }

// Start runs the worker.
func (d *deferInEndlessLoop) Start() {
	d.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run defers a Wait on every pass, and none of them runs.
func (d *deferInEndlessLoop) Run(tick func()) {
	for {
		defer d.wg.Wait()
		tick()
	}
}

// gotoBranchWait loops forever through goto and waits only on some passes.
type gotoBranchWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (g *gotoBranchWait) Start() {
	g.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run waits only when ready says so, forever.
func (g *gotoBranchWait) Run(ready func() bool) {
again:
	if ready() {
		g.wg.Wait()
	}
	goto again
}

// waitOnGiveUpOnly loops forever and waits only in its ctx.Done() case.
type waitOnGiveUpOnly struct {
	wg   sync.WaitGroup
	tick chan struct{}
}

// Start runs the worker.
func (w *waitOnGiveUpOnly) Start() {
	w.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run waits only once ctx is done, and keeps looping on ticks.
func (w *waitOnGiveUpOnly) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.wg.Wait()
		case <-w.tick:
		}
	}
}

// selectDefaultSkipsWait waits in one select case, and its default skips it.
type selectDefaultSkipsWait struct {
	wg   sync.WaitGroup
	tick chan struct{}
}

// Start runs the worker.
func (r *selectDefaultSkipsWait) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run loops forever.
func (r *selectDefaultSkipsWait) Run() {
	for {
		select {
		case <-r.tick:
			r.wg.Wait()
		default:
		}
	}
}

// switchSkipsWait has a switch with no default, which skips the Wait.
type switchSkipsWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *switchSkipsWait) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run loops forever.
func (r *switchSkipsWait) Run(v func() int) {
	for {
		switch v() {
		case 1:
			r.wg.Wait()
		}
	}
}

// continueSkipsWait continues the outer loop before it reaches the Wait.
type continueSkipsWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *continueSkipsWait) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run loops forever.
func (r *continueSkipsWait) Run(skip func() bool) {
outer:
	for {
		for {
			if skip() {
				continue outer
			}
			r.wg.Wait()
		}
	}
}

// exitIntoSpin may spin in its outer loop forever, since its only way out
// leads into another loop that never ends.
type exitIntoSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *exitIntoSpin) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run loops forever.
func (r *exitIntoSpin) Run(x func() bool) {
	for {
		if x() {
			for {
				r.wg.Wait()
			}
		}
	}
}

// rangeThenError ranges over a channel that may close, then returns an error
// with no Wait after the loop.
type rangeThenError struct {
	wg   sync.WaitGroup
	jobs chan func()
}

// Start runs the worker.
func (r *rangeThenError) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed drains the jobs and reports the group closed.
func (r *rangeThenError) Closed() error {
	for job := range r.jobs {
		job()
		r.wg.Wait()
	}
	return errClosed
}

// gotoRetryThenWait retries through goto, then waits on every pass.
//
// Expected false positive: only a for or range loop counts as a loop with a
// way out, so the goto retry is read as a pass that may repeat forever.
type gotoRetryThenWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *gotoRetryThenWait) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run loops forever.
func (r *gotoRetryThenWait) Run(ok func() bool) {
	for {
	retry:
		if !ok() {
			goto retry
		}
		r.wg.Wait()
	}
}
