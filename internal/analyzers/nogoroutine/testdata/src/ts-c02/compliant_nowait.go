// A function counts as a Wait when it calls Wait on the group, directly,
// through a chain of helpers that wait, inside a literal it calls, inside a
// t.Run body or inside a saved literal it calls, including when it then
// returns a stored error or only an error, or loops forever after the Wait.
package fixture

import (
	"sync"
	"testing"
)

// chainWaiter waits through two helpers, declared after the method that calls
// them.
type chainWaiter struct{ wg sync.WaitGroup }

// Start runs the worker.
func (c *chainWaiter) Start() { c.wg.Go(work) }

// Close waits through stop, which waits through join.
func (c *chainWaiter) Close() { c.stop() }

func (c *chainWaiter) stop() { c.join() }

func (c *chainWaiter) join() { c.wg.Wait() }

// literalWaiter waits inside a literal its exported method calls.
type literalWaiter struct{ wg sync.WaitGroup }

// Start runs the worker.
func (l *literalWaiter) Start() { l.wg.Go(work) }

// Close waits inside a called literal.
func (l *literalWaiter) Close() { func() { l.wg.Wait() }() }

// subtestWaiter waits inside a t.Run body of its exported method.
type subtestWaiter struct{ wg sync.WaitGroup }

// Start runs the worker.
func (s *subtestWaiter) Start() { s.wg.Go(work) }

// Close waits inside a subtest that does not call t.Parallel.
func (s *subtestWaiter) Close(t *testing.T) {
	t.Run("close", func(t *testing.T) { s.wg.Wait() })
}

// storedErrorHelperClose waits through a helper, then returns a stored error
// that may be non-nil.
type storedErrorHelperClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (s *storedErrorHelperClose) Start() { s.wg.Go(work) }

// Close waits through join and returns the stored error.
func (s *storedErrorHelperClose) Close() error {
	s.join()
	return s.err
}

func (s *storedErrorHelperClose) join() { s.wg.Wait() }

// storedErrorLiteralClose waits inside a called literal, then returns a stored
// error that may be non-nil.
type storedErrorLiteralClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (s *storedErrorLiteralClose) Start() { s.wg.Go(work) }

// Close waits inside a called literal and returns the stored error.
func (s *storedErrorLiteralClose) Close() error {
	func() { s.wg.Wait() }()
	return s.err
}

// waitThenSpin waits, then loops forever.
type waitThenSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (w *waitThenSpin) Start() { w.wg.Go(work) }

// Run waits for the worker before it loops forever.
func (w *waitThenSpin) Run() {
	w.wg.Wait()
	for {
	}
}

// savedWaitCalled waits inside a saved literal it calls, then returns only an
// error.
type savedWaitCalled struct{ wg sync.WaitGroup }

// Start runs the worker.
func (s *savedWaitCalled) Start() { s.wg.Go(work) }

// Closed waits through the saved literal and reports the group closed.
func (s *savedWaitCalled) Closed() error {
	wait := func() { s.wg.Wait() }
	wait()
	return errClosed
}
