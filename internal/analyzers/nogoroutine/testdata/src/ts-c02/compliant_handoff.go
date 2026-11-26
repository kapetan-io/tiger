// A Close that waits through sync.Once.Do, a method value of Wait, a helper
// handed the group, or a helper in another package counts as a Wait, also when
// it then returns a stored error and when it reaches the Wait through a
// same-package method.
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

// variadicClose hands its group to a variadic helper.
type variadicClose struct {
	other, wg sync.WaitGroup
	err       error
}

// Start runs the worker.
func (v *variadicClose) Start() { v.wg.Go(work) }

// Close waits through waitEach and returns the stored error.
func (v *variadicClose) Close() error {
	waitEach(&v.other, &v.wg)
	return v.err
}

func waitEach(groups ...*sync.WaitGroup) {
	for _, group := range groups {
		group.Wait()
	}
}

// secondArgClose hands its group to the second parameter of a helper whose
// first parameter never waits.
type secondArgClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (s *secondArgClose) Start() { s.wg.Go(work) }

// Close waits through waitSecond and returns the stored error.
func (s *secondArgClose) Close() error {
	waitSecond(&s.wg, &s.wg)
	return s.err
}

func waitSecond(_, second *sync.WaitGroup) { second.Wait() }

// stopOnceClose waits inside stop, a method with a success path.
type stopOnceClose struct {
	wg   sync.WaitGroup
	once sync.Once
	err  error
}

// Start runs the worker.
func (s *stopOnceClose) Start() { s.wg.Go(work) }

// Close stops and returns the stored error.
func (s *stopOnceClose) Close() error {
	s.stop()
	return s.err
}

func (s *stopOnceClose) stop() {
	s.once.Do(func() { s.wg.Wait() })
}

// stopValueClose waits inside stop, a method with a success path.
type stopValueClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (s *stopValueClose) Start() { s.wg.Go(work) }

// Close stops and returns the stored error.
func (s *stopValueClose) Close() error {
	s.stop()
	return s.err
}

func (s *stopValueClose) stop() {
	wait := s.wg.Wait
	wait()
}

// stopHandedClose waits inside stop, a method with a success path.
type stopHandedClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (s *stopHandedClose) Start() { s.wg.Go(work) }

// Close stops and returns the stored error.
func (s *stopHandedClose) Close() error {
	s.stop()
	return s.err
}

func (s *stopHandedClose) stop() {
	waitAll(&s.wg)
}

// stopRemoteClose waits inside stop, a method with a success path.
type stopRemoteClose struct {
	wg  sync.WaitGroup
	err error
}

// Start runs the worker.
func (s *stopRemoteClose) Start() { s.wg.Go(work) }

// Close stops and returns the stored error.
func (s *stopRemoteClose) Close() error {
	s.stop()
	return s.err
}

func (s *stopRemoteClose) stop() {
	stophelp.Wait(&s.wg)
}

// boundStopClose calls its stop method through a local bound to the method
// value.
type boundStopClose struct {
	wg   sync.WaitGroup
	once sync.Once
	err  error
}

// Start runs the worker.
func (b *boundStopClose) Start() { b.wg.Go(work) }

// Close stops through the bound method value and returns the stored error.
func (b *boundStopClose) Close() error {
	stop := b.stop
	stop()
	return b.err
}

func (b *boundStopClose) stop() { b.once.Do(func() { b.wg.Wait() }) }
