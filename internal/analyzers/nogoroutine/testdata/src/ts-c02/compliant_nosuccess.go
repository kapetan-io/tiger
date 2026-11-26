// A function with no success exit that waits on every exit it has, or on
// every path into a loop that never ends, counts as a Wait.
package fixture

import "sync"

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
