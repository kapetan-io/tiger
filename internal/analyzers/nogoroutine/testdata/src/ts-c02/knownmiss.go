// Documented TS-C02 coverage gaps.
package fixture

import (
	"context"
	"fmt"
	"os"
	"sync"
)

func startOnParameter(wg *sync.WaitGroup) { wg.Go(work) }

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

// helperBranchWait calls a same-package helper that waits on one branch only,
// and, in a function with no success path, a call counts when it may wait,
// not only when it always does.
type helperBranchWait struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (h *helperBranchWait) Start() { h.wg.Go(work) }

// Closed asks maybeWait not to wait and reports the group closed.
func (h *helperBranchWait) Closed() error {
	h.maybeWait(false)
	return errClosed
}

func (h *helperBranchWait) maybeWait(wait bool) {
	if wait {
		h.wg.Wait()
	}
}

// literalBranchWait runs a literal that waits on one branch only, and, in a
// function with no success path, a call counts when it may wait, not only
// when it always does.
type literalBranchWait struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (l *literalBranchWait) Start() { l.wg.Go(work) }

// Closed runs the literal and reports the group closed.
func (l *literalBranchWait) Closed(wait bool) error {
	func() {
		if wait {
			l.wg.Wait()
		}
	}()
	return errClosed
}

// deferPanicForever never runs its deferred Wait unless it panics, and a
// path that ends in panic is read as a success path the defer covers.
type deferPanicForever struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (d *deferPanicForever) Start() { d.wg.Go(work) }

// Run defers a Wait and loops forever, panicking when bad says so.
func (d *deferPanicForever) Run(bad func() bool) {
	defer d.wg.Wait()
	for {
		if bad() {
			panic("bad")
		}
	}
}

// deferSelectForever blocks forever in select {}, which is read as a success
// path the deferred Wait covers.
type deferSelectForever struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (d *deferSelectForever) Start() { d.wg.Go(work) }

// Run defers a Wait and blocks forever.
func (d *deferSelectForever) Run() {
	defer d.wg.Wait()
	select {}
}

// branchInLoop waits only on some passes of a loop that never ends.
type branchInLoop struct{ wg sync.WaitGroup }

// Start runs the worker.
//
// known-miss: a function that never returns counts as a Wait when it may
// reach a Wait outside a defer on some path, so a loop that waits on one
// branch, case or pass only still counts; the cases below share this note.
func (b *branchInLoop) Start() { b.wg.Go(work) }

// Run waits only when ready says so, forever.
func (b *branchInLoop) Run(ready func() bool) {
	for {
		if ready() {
			b.wg.Wait()
		}
	}
}

// gotoBranchWait loops forever through goto and waits only on some passes.
type gotoBranchWait struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (g *gotoBranchWait) Start() { g.wg.Go(work) }

// Run waits only when ready says so, forever.
func (g *gotoBranchWait) Run(ready func() bool) {
again:
	if ready() {
		g.wg.Wait()
	}
	goto again
}

// waitOnGiveUpOnly loops forever and waits only in its ctx.Done() case.
type waitOnGiveUpOnly struct {
	wg   sync.WaitGroup
	tick chan struct{}
}

// Start runs the worker. Not reported.
func (w *waitOnGiveUpOnly) Start() { w.wg.Go(work) }

// Run waits only once ctx is done, and keeps looping on ticks.
func (w *waitOnGiveUpOnly) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.wg.Wait()
		case <-w.tick:
		}
	}
}

// selectDefaultSkipsWait waits in one select case, and its default skips it.
type selectDefaultSkipsWait struct {
	wg   sync.WaitGroup
	tick chan struct{}
}

// Start runs the worker. Not reported.
func (r *selectDefaultSkipsWait) Start() { r.wg.Go(work) }

// Run loops forever.
func (r *selectDefaultSkipsWait) Run() {
	for {
		select {
		case <-r.tick:
			r.wg.Wait()
		default:
		}
	}
}

// switchSkipsWait has a switch with no default, which skips the Wait.
type switchSkipsWait struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (r *switchSkipsWait) Start() { r.wg.Go(work) }

// Run loops forever.
func (r *switchSkipsWait) Run(v func() int) {
	for {
		switch v() {
		case 1:
			r.wg.Wait()
		}
	}
}

// continueSkipsWait continues the outer loop before it reaches the Wait.
type continueSkipsWait struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (r *continueSkipsWait) Start() { r.wg.Go(work) }

// Run loops forever.
func (r *continueSkipsWait) Run(skip func() bool) {
outer:
	for {
		for {
			if skip() {
				continue outer
			}
			r.wg.Wait()
		}
	}
}

// exitIntoSpin may spin in its outer loop forever, since its only way out
// leads into another loop that never ends.
type exitIntoSpin struct{ wg sync.WaitGroup }

// Start runs the worker. Not reported.
func (r *exitIntoSpin) Start() { r.wg.Go(work) }

// Run loops forever.
func (r *exitIntoSpin) Run(x func() bool) {
	for {
		if x() {
			for {
				r.wg.Wait()
			}
		}
	}
}
