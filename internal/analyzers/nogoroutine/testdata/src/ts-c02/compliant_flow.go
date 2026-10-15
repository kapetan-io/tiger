// Waits that are reached: on both branches, in a defer, in a helper that
// always waits, in a literal that is called, and on every success path of a
// Shutdown that returns an error before it waits.
package fixture

import (
	"errors"
	"sync"
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
