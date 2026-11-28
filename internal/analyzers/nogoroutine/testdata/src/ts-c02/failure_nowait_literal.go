// A function with no success path counts as a Wait only for a Wait it reaches
// before it returns. A parallel subtest started on the function's own t runs
// after the function returns, a saved literal replaced before its call runs
// the replacement, and a function that only calls t.Fatal waits on nothing, so
// the starts that rely on each are reported.
package fixture

import (
	"sync"
	"testing"
)

// parallelOwnT waits only in a parallel subtest of its own t.
type parallelOwnT struct{ wg sync.WaitGroup }

// Start runs the worker.
func (p *parallelOwnT) Start() {
	p.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Drain returns before its parallel subtest runs, then fails.
func (p *parallelOwnT) Drain(t *testing.T) error {
	t.Run("wait", func(t *testing.T) {
		t.Parallel()
		p.wg.Wait()
	})
	return errClosed
}

// replacedBeforeCall replaces its saved Wait literal before it calls it.
type replacedBeforeCall struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *replacedBeforeCall) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed calls the replacement, which waits on nothing, and reports the group
// closed.
func (r *replacedBeforeCall) Closed() error {
	wait := func() { r.wg.Wait() }
	wait = func() {}
	wait()
	return errClosed
}

// fatalOnly fails the test on one path and returns an error on the other.
type fatalOnly struct{ wg sync.WaitGroup }

// Start runs the worker.
func (f *fatalOnly) Start() {
	f.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed fails the test under -short and otherwise reports the group closed.
func (f *fatalOnly) Closed(t *testing.T) error {
	if testing.Short() {
		t.Fatal("short")
	}
	return errClosed
}

// closedFatal fails the test, then returns an error.
func closedFatal(t *testing.T) error {
	t.Fatal("closed")
	return errClosed
}

// startThenFatalOnly calls a function that only fails the test after the
// start, and nothing waits.
func startThenFatalOnly(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	_ = closedFatal(t)
}
