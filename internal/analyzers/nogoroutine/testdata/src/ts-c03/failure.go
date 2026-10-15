// The TS-C03 failure mode: a package whose files start a goroutine and whose
// tests have no goleak check, so a goroutine that outlives its owner passes.
package fixture

import "sync"

// Pool starts a worker the group waits for, so TS-C02 is satisfied and only
// the missing leak check remains.
type Pool struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (p *Pool) Start() {
	p.wg.Go(func() {}) // want `TS-C03: this package starts goroutines but its tests have no goleak check`
}

// Close waits for the worker.
func (p *Pool) Close() { p.wg.Wait() }

// StartAgain runs another worker; only the first start is reported.
func (p *Pool) StartAgain() {
	p.wg.Go(func() {})
}
