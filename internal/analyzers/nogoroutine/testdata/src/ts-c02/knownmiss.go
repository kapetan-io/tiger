// Documented TS-C02 coverage gaps.
package fixture

import (
	"context"
	"fmt"
	"os"
	"sync"
)

func startOnParameter(wg *sync.WaitGroup) { wg.Go(work) }

// replacedOnBranch starts a worker, and its Closed may replace the waiting
// literal before calling it.
type replacedOnBranch struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *replacedOnBranch) Start() { r.wg.Go(work) }

// Closed waits unless the program got an argument, then reports the group
// closed.
//
// known-miss: a function with no success path counts when any literal that
// may reach the call waits, the same as a Wait on one branch only.
func (r *replacedOnBranch) Closed() error {
	wait := func() { r.wg.Wait() }
	if len(os.Args) > 1 {
		wait = func() {}
	}
	wait()
	return errClosed
}

// callerNeverWaits passes its group to a function that starts a goroutine on
// it and never waits.
//
// known-miss: a group received as a parameter is the caller's to wait on, and
// this check does not follow the argument back to see that the caller does
// not.
func callerNeverWaits() {
	var wg sync.WaitGroup
	startOnParameter(&wg)
}

// waitOnEarlyPath waits only on a path that returns before the start, then
// gives up through a ctx.Done() case after it.
//
// known-miss: any Wait on the group in the function lets a ctx.Done() case
// after the start stand in for one, even a Wait the path never reaches or
// reaches only before the start (waitBeforeGiveUp, expiredWaitBeforeGiveUp).
func waitOnEarlyPath(ctx context.Context, early bool) {
	var wg sync.WaitGroup
	if early {
		wg.Wait()
		return
	}
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}

// waitBeforeGiveUp waits before the start, then gives up through a ctx.Done()
// case after it. The known-miss note on waitOnEarlyPath covers it.
func waitBeforeGiveUp(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Wait()
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}

// expiredWaitBeforeGiveUp waits only in a ctx.Done() case before the start,
// then gives up through another after it. The known-miss note on
// waitOnEarlyPath covers it.
func expiredWaitBeforeGiveUp(ctx context.Context) {
	var wg sync.WaitGroup
	select {
	case <-ctx.Done():
		wg.Wait()
	default:
	}
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}

// branchWaitSentinel leaks the goroutine when Closed is called with false.
type branchWaitSentinel struct{ wg sync.WaitGroup }

// Start runs the worker.
//
// known-miss: Closed returns only a non-nil error, so it has no success path
// to check, and its Wait on one branch makes it count as a Wait on every path (ENG-980).
func (b *branchWaitSentinel) Start() { b.wg.Go(work) }

// Closed waits only when asked to, then reports the group closed.
func (b *branchWaitSentinel) Closed(wait bool) error {
	if wait {
		b.wg.Wait()
	}
	return errClosed
}

// deferForever never runs its deferred Wait, because Run never returns.
type deferForever struct{ wg sync.WaitGroup }

// Start runs the worker.
//
// known-miss: Run has no success path to check, and its deferred Wait makes it
// count as a Wait though the Wait never runs (ENG-980).
func (d *deferForever) Start() { d.wg.Go(work) }

// Run defers a Wait and loops forever.
func (d *deferForever) Run() {
	defer d.wg.Wait()
	for {
	}
}

// spinBeforeWait never reaches its Wait, because spin never returns.
type spinBeforeWait struct{ wg sync.WaitGroup }

// Start runs the worker.
//
// known-miss: a call is treated as returning unless it is the builtin panic,
// so the Wait after spin counts though it never runs.
func (s *spinBeforeWait) Start() { s.wg.Go(work) }

// Run spins forever before it waits.
func (s *spinBeforeWait) Run() {
	spin()
	s.wg.Wait()
}

func spin() {
	for {
	}
}

// exitBeforeWait never reaches its Wait, because os.Exit never returns.
type exitBeforeWait struct{ wg sync.WaitGroup }

// Start runs the worker.
//
// known-miss: os.Exit is treated as returning, so the Wait after it counts
// though it never runs.
func (e *exitBeforeWait) Start() { e.wg.Go(work) }

// Stop exits the process before it waits.
func (e *exitBeforeWait) Stop() {
	os.Exit(1)
	e.wg.Wait()
}

// onceAlreadyFired waits inside a once.Do whose Once its Start already fired,
// so the Wait never runs. Nothing tracks whether a Once has fired.
type onceAlreadyFired struct {
	wg   sync.WaitGroup
	once sync.Once
	err  error
}

// Start fires the Once, then runs the worker. Not reported.
func (o *onceAlreadyFired) Start() {
	o.once.Do(func() {})
	o.wg.Go(work)
}

// Close waits inside once.Do, which no longer runs its literal.
func (o *onceAlreadyFired) Close() error {
	o.once.Do(func() { o.wg.Wait() })
	return o.err
}

// printedForever hands its group to fmt.Println, a call the check cannot
// follow, so it counts as a Wait though Println never waits.
type printedForever struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (p *printedForever) Start() { p.wg.Go(work) }

// Run prints the group, then loops forever.
func (p *printedForever) Run() {
	fmt.Println(&p.wg)
	for {
	}
}

// addOnlyValue hands its group to a func value that only calls Add.
type addOnlyValue struct {
	wg    sync.WaitGroup
	touch func(*sync.WaitGroup)
}

// Start runs the worker. Not reported.
func (a *addOnlyValue) Start() { a.wg.Go(work) }

// Closed touches the group and reports the group closed.
func (a *addOnlyValue) Closed() error {
	a.touch(&a.wg)
	return errClosed
}

// variadicLogged hands its group to a same-package variadic function that
// never waits; a variadic parameter is not followed.
type variadicLogged struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (v *variadicLogged) Start() { v.wg.Go(work) }

// Closed logs the group and reports the group closed.
func (v *variadicLogged) Closed() error {
	logf("closing %v", &v.wg)
	return errClosed
}

func logf(format string, args ...any) { _ = fmt.Sprintf(format, args...) }

// otherInstance waits on the group of a different value of its own type, which
// counts because a group is matched by its field, not by the value holding it.
type otherInstance struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (o *otherInstance) Start() { o.wg.Go(work) }

// Closed stops a fresh value and reports the group closed.
func (o *otherInstance) Closed() error {
	var fresh otherInstance
	fresh.stop()
	return errClosed
}

func (o *otherInstance) stop() { o.wg.Wait() }
