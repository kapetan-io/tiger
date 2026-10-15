// Naming goleak is not checking for leaks: a TestMain that only mentions an
// option does not call VerifyTestMain.
package fixture

import "sync"

// Pool starts a worker the group waits for.
type Pool struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (p *Pool) Start() {
	p.wg.Go(func() {}) // want `TS-C03: this package starts goroutines but its tests have no goleak check`
}

// Close waits for the worker.
func (p *Pool) Close() { p.wg.Wait() }
