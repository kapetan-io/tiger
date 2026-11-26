// Expected false TS-C02 findings: a path that ends in panic never returns, so
// it needs no Wait, but it is read as a success exit (ENG-998).
package fixture

import "sync"

// panicThenError panics on one branch and waits before its error return on
// the other.
type panicThenError struct{ wg sync.WaitGroup }

// Start runs the worker.
func (p *panicThenError) Start() {
	p.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed panics when bad, and otherwise waits and reports the group closed.
func (p *panicThenError) Closed(bad bool) error {
	if bad {
		panic("bad")
	}
	p.wg.Wait()
	return errClosed
}

// panicThenWait panics on one branch and waits before its success return on
// the other.
type panicThenWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (p *panicThenWait) Start() {
	p.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed panics when bad, and otherwise waits.
func (p *panicThenWait) Closed(bad bool) error {
	if bad {
		panic("bad")
	}
	p.wg.Wait()
	return nil
}
