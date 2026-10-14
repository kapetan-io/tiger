// A Wait counts only when it is reached on the success path: not after a
// return, not on one branch, not inside a literal nobody calls, and not from
// the goroutine the group is waiting for.
package fixture

import "sync"

// unreachableWait has a Wait after a return.
type unreachableWait struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (s *unreachableWait) Start() {
	s.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Close returns before it waits.
func (s *unreachableWait) Close() {
	return
	s.wg.Wait()
}

// oneBranchWait waits only when asked to.
type oneBranchWait struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (v *oneBranchWait) Start() {
	v.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Close waits on one branch, and returns on the other without waiting.
func (v *oneBranchWait) Close(skip bool) {
	if skip {
		v.wg.Wait()
	}
}

func conditionalWait(n int) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if n > 0 {
		wg.Wait()
	}
}

func waitsOnItself() {
	var wg sync.WaitGroup
	wg.Go(func() { wg.Wait() }) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// literalNeverCalled builds a function that waits and drops it.
type literalNeverCalled struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (u *literalNeverCalled) Start() {
	u.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Stop never calls the literal.
func (u *literalNeverCalled) Stop() {
	wait := func() { u.wg.Wait() }
	_ = wait
}

// waitBeforeGo waits before the goroutine starts, so the Wait joins nothing.
func waitBeforeGo() {
	var wg sync.WaitGroup
	wg.Wait()
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}
