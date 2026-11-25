// A Close that waits in a way the check cannot follow, then returns a stored
// error, keeps counting as a Wait: through sync.Once.Do, a method value of
// Wait, a helper handed the group, or a helper in another package.
package fixture

import (
	"sync"

	"stophelp"
)

// onceClose waits once, inside sync.Once.Do.
type onceClose struct {
	wg   sync.WaitGroup
	once sync.Once
	err  error
}

// Start runs the worker.
func (o *onceClose) Start() { o.wg.Go(work) }

// Close waits inside once.Do and returns the stored error.
func (o *onceClose) Close() error {
	o.once.Do(func() { o.wg.Wait() })
	return o.err
}

// valueClose waits through a method value of Wait.
type valueClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (v *valueClose) Start() { v.wg.Go(work) }

// Close calls the saved Wait and returns the stored error.
func (v *valueClose) Close() error {
	wait := v.wg.Wait
	wait()
	return v.err
}

// handedClose hands its group to a helper that waits on it.
type handedClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (h *handedClose) Start() { h.wg.Go(work) }

// Close waits through waitAll and returns the stored error.
func (h *handedClose) Close() error {
	waitAll(&h.wg)
	return h.err
}

func waitAll(wg *sync.WaitGroup) { wg.Wait() }

// remoteClose hands its group to a helper in another package.
type remoteClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (r *remoteClose) Start() { r.wg.Go(work) }

// Close waits through stophelp.Wait and returns the stored error.
func (r *remoteClose) Close() error {
	stophelp.Wait(&r.wg)
	return r.err
}
