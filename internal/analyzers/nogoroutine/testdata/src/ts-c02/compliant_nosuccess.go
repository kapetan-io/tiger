// A function with no success exit that waits before every return it has, or
// before or on every pass of each loop it can never leave, counts as a Wait.
package fixture

import (
	"context"
	"sync"
)

// deferThenErrors defers its Wait before any return.
type deferThenErrors struct{ wg sync.WaitGroup }

// Start runs the worker.
func (d *deferThenErrors) Start() { d.wg.Go(work) }

// Closed waits on every return through the defer.
func (d *deferThenErrors) Closed(flush func() error) error {
	defer d.wg.Wait()
	if err := flush(); err != nil {
		return err
	}
	return errClosed
}

// stopLoop runs until stopped, and waits before it reports the stop.
type stopLoop struct {
	wg   sync.WaitGroup
	stop chan struct{}
	jobs chan func()
}

// Start runs the worker.
func (s *stopLoop) Start() { s.wg.Go(work) }

// Run handles jobs until stop closes; its only return waits first.
func (s *stopLoop) Run() error {
	for {
		select {
		case <-s.stop:
			s.wg.Wait()
			return errClosed
		case job := <-s.jobs:
			job()
		}
	}
}

// waitEachPass waits on every pass of a loop that never ends.
type waitEachPass struct{ wg sync.WaitGroup }

// Start runs the worker.
func (w *waitEachPass) Start() { w.wg.Go(work) }

// Run waits, then spins, forever.
func (w *waitEachPass) Run(tick func()) {
	for {
		w.wg.Wait()
		tick()
	}
}

// branchesBothWait waits on both branches, then reports the group closed.
type branchesBothWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (b *branchesBothWait) Start() { b.wg.Go(work) }

// Closed waits on either branch.
func (b *branchesBothWait) Closed(fast bool) error {
	if fast {
		b.wg.Wait()
		return errClosed
	}
	b.wg.Wait()
	return errClosed
}

// namedWaitFirst waits before either return of its named error result.
type namedWaitFirst struct{ wg sync.WaitGroup }

// Start runs the worker.
func (n *namedWaitFirst) Start() { n.wg.Go(work) }

// Closed waits, then returns the set error on either branch.
func (n *namedWaitFirst) Closed(c bool) (err error) {
	err = errClosed
	n.wg.Wait()
	if c {
		return err
	}
	return err
}

// waitThenInnerSpin waits before it enters an inner loop that never ends.
type waitThenInnerSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (w *waitThenInnerSpin) Start() { w.wg.Go(work) }

// Run waits, then spins in the inner loop forever.
func (w *waitThenInnerSpin) Run(tick func()) {
	for {
		w.wg.Wait()
		for {
			tick()
		}
	}
}

// deferInLoopThenReturn defers its Wait in a loop before the loop returns.
type deferInLoopThenReturn struct{ wg sync.WaitGroup }

// Start runs the worker.
func (d *deferInLoopThenReturn) Start() { d.wg.Go(work) }

// Run defers the Wait before its only return.
func (d *deferInLoopThenReturn) Run(done func() bool) error {
	for {
		defer d.wg.Wait()
		if done() {
			return errClosed
		}
	}
}

// gotoWaitEachPass loops forever through goto and waits on every pass.
type gotoWaitEachPass struct{ wg sync.WaitGroup }

// Start runs the worker.
func (g *gotoWaitEachPass) Start() { g.wg.Go(work) }

// Run waits on every pass.
func (g *gotoWaitEachPass) Run() {
again:
	g.wg.Wait()
	goto again
}

// giveUpSentinel waits when done, or gives up through a ctx.Done() case that
// returns the same sentinel error.
type giveUpSentinel struct {
	wg   sync.WaitGroup
	done chan struct{}
}

// Start runs the worker.
func (g *giveUpSentinel) Start() { g.wg.Go(work) }

// Closed waits for the worker, or for ctx.
func (g *giveUpSentinel) Closed(ctx context.Context) error {
	select {
	case <-g.done:
		g.wg.Wait()
		return errClosed
	case <-ctx.Done():
		return errClosed
	}
}

// returnOrSpin waits before its only return, and spins forever on another
// path; a function with some return is judged by its returns alone.
type returnOrSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *returnOrSpin) Start() { r.wg.Go(work) }

// Run spins forever when c is set, and otherwise waits and reports closed.
func (r *returnOrSpin) Run(c bool) error {
	if c {
		for {
		}
	}
	r.wg.Wait()
	return errClosed
}

// siblingCallWait waits in a literal whose statement then makes another call.
type siblingCallWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (s *siblingCallWait) Start() { s.wg.Go(work) }

// Closed waits through the literal, then counts, in one statement.
func (s *siblingCallWait) Closed(count func() int) error {
	err, n := func() error { s.wg.Wait(); return errClosed }(), count()
	_ = n
	return err
}

// siblingCallSpin waits each pass in a literal whose statement then makes
// another call.
type siblingCallSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (s *siblingCallSpin) Start() { s.wg.Go(work) }

// Run waits through the literal and counts, forever.
func (s *siblingCallSpin) Run(count func() int) {
	for {
		_, n := func() error { s.wg.Wait(); return errClosed }(), count()
		_ = n
	}
}

// boundedThenSpin runs a loop that ends before the loop that waits forever.
type boundedThenSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (b *boundedThenSpin) Start() { b.wg.Go(work) }

// Run warms up three times, then waits on every pass forever.
func (b *boundedThenSpin) Run(tick func()) {
	for i := 0; i < 3; i++ {
		tick()
	}
	for {
		b.wg.Wait()
		tick()
	}
}
