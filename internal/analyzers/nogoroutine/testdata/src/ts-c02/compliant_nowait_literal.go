// A function with no success path counts as a Wait when it reaches the Wait
// before it returns, even if it later replaces the saved literal that held
// the Wait.
package fixture

import "sync"

// replacedAfterCall replaces its saved Wait literal after it calls it.
type replacedAfterCall struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *replacedAfterCall) Start() { r.wg.Go(work) }

// Closed waits through the saved literal, replaces it, and reports the group
// closed.
func (r *replacedAfterCall) Closed() error {
	wait := func() { r.wg.Wait() }
	wait()
	wait = func() {}
	return errClosed
}
