// A function whose only Wait does not run before it returns (in a goroutine,
// in a literal saved and never called, or after a loop that never ends) waits
// on nothing, even when it has no success exit.
package fixture

import "sync"

// goWaitSpin waits only inside a goroutine, then loops forever.
type goWaitSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (g *goWaitSpin) Start() {
	g.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run waits in a goroutine of its own and loops forever.
func (g *goWaitSpin) Run() {
	go func() { g.wg.Wait() }() // want `TS-C02: this go statement starts a goroutine nobody owns`
	for {
	}
}

// savedWaitSentinel saves a waiting literal it never calls, then returns only
// an error.
type savedWaitSentinel struct{ wg sync.WaitGroup }

// Start runs the worker.
func (s *savedWaitSentinel) Start() {
	s.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed returns only an error; its Wait is never called.
func (s *savedWaitSentinel) Closed() error {
	wait := func() { s.wg.Wait() }
	_ = wait
	return errClosed
}

// waitAfterSpin has its Wait after a loop that never ends.
type waitAfterSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (w *waitAfterSpin) Start() {
	w.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run loops forever; the Wait after the loop is never reached.
func (w *waitAfterSpin) Run() {
	for {
	}
	w.wg.Wait()
}

// startThenSpin starts on a local group and calls a function that loops
// forever and never waits.
func startThenSpin() {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	Spin()
}
