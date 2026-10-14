// The TS-C03 compliant shape: goroutines start through a group that is waited
// for, and a TestMain in the package's external test package checks for leaks.
package fixture

import "sync"

// Pool starts a worker the group waits for.
type Pool struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (p *Pool) Start() {
	p.wg.Go(func() {})
}

// Close waits for the worker.
func (p *Pool) Close() { p.wg.Wait() }
