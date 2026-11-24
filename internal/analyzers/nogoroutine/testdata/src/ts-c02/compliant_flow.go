// Waits that are reached: on both branches, in a defer, in a helper that
// always waits, in a literal that is called, and on every success path of a
// Shutdown that returns an error before it waits or gives up through a
// ctx.Done() case after the start.
package fixture

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
)

func waitOnBothBranches(n int) {
	var wg sync.WaitGroup
	wg.Go(work)
	if n > 0 {
		wg.Wait()
	} else {
		wg.Wait()
	}
}

func deferredWaitAfterGo() {
	var wg sync.WaitGroup
	wg.Go(work)
	defer wg.Wait()
}

func deferredWaitBeforeGo(n int) {
	var wg sync.WaitGroup
	defer wg.Wait()
	if n > 0 {
		return
	}
	wg.Go(work)
}

func waitInCalledLiteral() {
	var wg sync.WaitGroup
	wg.Go(work)
	func() { wg.Wait() }()
}

func waitInDeferredLiteral() {
	var wg sync.WaitGroup
	wg.Go(work)
	defer func() { wg.Wait() }()
}

// helperWaiter waits through a helper that always waits.
type helperWaiter struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (h *helperWaiter) Start() { h.wg.Go(work) }

// Close waits through join.
func (h *helperWaiter) Close() { h.join() }

func (h *helperWaiter) join() { h.wg.Wait() }

var errFlush = errors.New("flush failed")

// errorBeforeWait returns an error before it waits, which is not the success
// path.
type errorBeforeWait struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (e *errorBeforeWait) Start() { e.wg.Go(work) }

// Shutdown waits only when the flush worked.
func (e *errorBeforeWait) Shutdown(flush func() error) error {
	if err := flush(); err != nil {
		return err
	}
	e.wg.Wait()
	return nil
}

// expiredAfterGo gives up through a ctx.Done() case after the start, which is
// the exempt early return; unlike Loop.Shutdown, the group is local.
func expiredAfterGo(ctx context.Context, done chan struct{}) error {
	var wg sync.WaitGroup
	wg.Go(work)
	select {
	case <-done:
		wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// expiredThenWaitedTryGo passes a ctx.Done() case before TryGo, then waits.
func expiredThenWaitedTryGo(ctx context.Context) error {
	var g errgroup.Group
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	g.TryGo(func() error { return nil })
	return g.Wait()
}

// expiredThenWaitedGo passes a ctx.Done() case before Go, then waits.
func expiredThenWaitedGo(ctx context.Context) error {
	var wg sync.WaitGroup
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	wg.Go(work)
	wg.Wait()
	return nil
}

// expiredBeforeStartLoop checks ctx before it starts, and its Shutdown, a
// different method, gives up through a ctx.Done() case; that early return is
// exempt.
type expiredBeforeStartLoop struct {
	wg   sync.WaitGroup
	stop chan struct{}
	done chan struct{}
}

// Start returns early when ctx has expired, otherwise runs the loop.
func (l *expiredBeforeStartLoop) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	l.stop, l.done = make(chan struct{}), make(chan struct{})
	l.wg.Go(func() { <-l.stop; close(l.done) })
	return nil
}

// Shutdown starts the stop and waits for the loop, or for ctx.
func (l *expiredBeforeStartLoop) Shutdown(ctx context.Context) error {
	close(l.stop)
	select {
	case <-l.done:
		l.wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// storedErrorClose waits and returns a stored error, which may be non-nil, or
// gives up through a ctx.Done() case.
type storedErrorClose struct {
	wg   sync.WaitGroup
	done chan struct{}
	err  error
}

// Start runs the worker.
func (s *storedErrorClose) Start() { s.wg.Go(work) }

// Close waits for the worker, or for ctx.
func (s *storedErrorClose) Close(ctx context.Context) error {
	select {
	case <-s.done:
		s.wg.Wait()
		return s.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// expiredNested returns from a ctx.Done() case nested in another.
func expiredNested(ctx, inner context.Context, done chan struct{}) error {
	var wg sync.WaitGroup
	wg.Go(work)
	select {
	case <-done:
		wg.Wait()
		return nil
	case <-ctx.Done():
		select {
		case <-inner.Done():
			return inner.Err()
		}
	}
}

// expiredAfterWaitedTryGo returns through a ctx.Done() case after TryGo, and
// waits on the other path.
func expiredAfterWaitedTryGo(ctx context.Context, done chan struct{}) error {
	var g errgroup.Group
	if !g.TryGo(func() error { return nil }) {
		return nil
	}
	select {
	case <-done:
		return g.Wait()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// expiredFallsThroughAfterGo gives up through a ctx.Done() case that falls
// through to the end, and waits on the other path.
func expiredFallsThroughAfterGo(ctx context.Context, ready chan struct{}) error {
	var wg sync.WaitGroup
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	case <-ready:
		wg.Wait()
	}
	return nil
}

// fallThroughStop has a Stop that gives up by falling through its ctx.Done()
// case.
type fallThroughStop struct {
	wg   sync.WaitGroup
	done chan struct{}
}

// Start runs the worker.
func (f *fallThroughStop) Start() { f.wg.Go(work) }

// Stop waits for the worker, or stops waiting when ctx expires.
func (f *fallThroughStop) Stop(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	case <-f.done:
		f.wg.Wait()
	}
}

// dispatchLoop starts a goroutine per job and waits in the ctx.Done() case,
// its only exit.
func dispatchLoop(ctx context.Context, jobs chan func()) error {
	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case job := <-jobs:
			wg.Go(job)
		}
	}
}

// dispatcher is dispatchLoop on a field group; Run is the type's only waiter.
type dispatcher struct {
	wg   sync.WaitGroup
	jobs chan func()
}

// Run starts a goroutine per job and waits when ctx expires.
func (d *dispatcher) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			d.wg.Wait()
			return nil
		case job := <-d.jobs:
			d.wg.Go(job)
		}
	}
}

// waitInExpired starts and then waits inside the only case, on ctx.Done().
func waitInExpired(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(work)
	select {
	case <-ctx.Done():
		wg.Wait()
	}
}
