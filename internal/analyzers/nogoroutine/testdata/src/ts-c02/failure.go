// The TS-C02 failure modes: goroutines with no owner, and goroutines started
// through a group nothing waits on.
package fixture

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

func launch() {
	go work() // want `TS-C02: this go statement starts a goroutine nobody owns`
}

// launchClosure spawns from inside a closure; the owner is still
// launchClosure, the FuncDecl the closure lexically sits in.
func launchClosure() {
	fn := func() {
		go work() // want `TS-C02: this go statement starts a goroutine nobody owns`
	}
	fn()
}

// UnwaitedDaemon has querator's trial shape: Add, go and Done, but Shutdown
// never waits, so it returns while the goroutine still runs.
type UnwaitedDaemon struct {
	wg   sync.WaitGroup
	stop chan struct{}
}

// Start runs the server goroutine.
func (d *UnwaitedDaemon) Start() {
	d.wg.Add(1)
	go func() { // want `TS-C02: this go statement starts a goroutine nobody owns`
		defer d.wg.Done()
		<-d.stop
	}()
}

// Shutdown signals the goroutine and returns without waiting for it.
func (d *UnwaitedDaemon) Shutdown() { close(d.stop) }

// GoNoWaitDaemon uses wg.Go and never waits: the trial bug in a form a check
// that only looked at go statements would accept.
type GoNoWaitDaemon struct {
	wg   sync.WaitGroup
	stop chan struct{}
}

// Start runs the server goroutine.
func (d *GoNoWaitDaemon) Start() {
	d.wg.Go(func() { <-d.stop }) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Shutdown signals the goroutine and returns without waiting.
func (d *GoNoWaitDaemon) Shutdown() { close(d.stop) }

func errgroupNoWait() {
	var g errgroup.Group
	g.Go(func() error { work(); return nil }) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

func errgroupTryGoNoWait() {
	var g errgroup.Group
	g.TryGo(func() error { work(); return nil }) // want `TS-C02: this TryGo call starts a goroutine, but no Wait on the group is reached`
}

// waitsOnAnotherGroup waits on a different group than the one it started.
func waitsOnAnotherGroup() {
	var started, other sync.WaitGroup
	started.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	other.Wait()
}

// Hidden waits in a method no exported method reaches.
type Hidden struct {
	wg sync.WaitGroup
}

// Start runs the worker.
func (h *Hidden) Start() {
	h.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

func (h *Hidden) join() { h.wg.Wait() }

// Impatient waits only on the path where its caller's ctx expired.
type Impatient struct {
	wg   sync.WaitGroup
	done chan struct{}
}

// Start runs the worker.
func (i *Impatient) Start() {
	i.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Shutdown waits for the worker only after ctx has expired.
func (i *Impatient) Shutdown(ctx context.Context) error {
	select {
	case <-i.done:
		return nil
	case <-ctx.Done():
		i.wg.Wait()
		return ctx.Err()
	}
}

func fromResult(next func() *sync.WaitGroup) {
	next().Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}
