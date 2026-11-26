// A function whose only Wait does not run before it returns (in a goroutine,
// in a literal or method value saved and never called, or after a loop that
// never ends) waits on nothing, even when it has no success exit.
package fixture

import (
	"sync"
	"time"
)

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

// otherGroupWait waits only inside a goroutine of another group, then loops
// forever.
type otherGroupWait struct{ a, b sync.WaitGroup }

// Start runs the worker.
func (o *otherGroupWait) Start() {
	o.a.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run waits on a inside b's goroutine and loops forever, so b needs no Wait.
func (o *otherGroupWait) Run() {
	o.b.Go(func() { o.a.Wait() })
	for {
	}
}

// trackedForever hands its group to a helper that never waits on it.
type trackedForever struct{ wg sync.WaitGroup }

// Start runs the worker.
func (t *trackedForever) Start() {
	t.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Serve tracks the group forever and never waits.
func (t *trackedForever) Serve() {
	for {
		track(&t.wg)
	}
}

func track(wg *sync.WaitGroup) {
	wg.Add(1)
	wg.Done()
}

// laterWait waits only in a literal time.AfterFunc runs after Closed returns.
type laterWait struct{ wg sync.WaitGroup }

// Start runs the worker.
func (l *laterWait) Start() {
	l.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Closed schedules a Wait for later and reports the group closed.
func (l *laterWait) Closed() error {
	time.AfterFunc(time.Second, func() { l.wg.Wait() })
	return errClosed
}

// onceSuccessClose has a success path, so its once.Do literal is not followed
// and its Close waits on nothing.
type onceSuccessClose struct {
	wg   sync.WaitGroup
	once sync.Once
}

// Start runs the worker.
func (o *onceSuccessClose) Start() {
	o.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Close waits inside once.Do and returns nil.
func (o *onceSuccessClose) Close() error {
	o.once.Do(func() { o.wg.Wait() })
	return nil
}

// laterValue hands its Wait method value to time.AfterFunc, which calls it
// later.
type laterValue struct{ wg sync.WaitGroup }

// Start runs the worker.
func (l *laterValue) Start() { l.wg.Go(work) } // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`

// Closed schedules the Wait and reports the group closed.
func (l *laterValue) Closed() error {
	time.AfterFunc(time.Second, l.wg.Wait)
	return errClosed
}

// otherGroupValue runs its Wait method value in another group's goroutine.
type otherGroupValue struct{ a, b sync.WaitGroup }

// Start runs the worker.
func (o *otherGroupValue) Start() { o.a.Go(work) } // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`

// Run waits on a inside b's goroutine, then loops forever.
func (o *otherGroupValue) Run() {
	o.b.Go(o.a.Wait)
	for {
	}
}

// savedValue takes its Wait method value and never calls it.
type savedValue struct{ wg sync.WaitGroup }

// Start runs the worker.
func (s *savedValue) Start() { s.wg.Go(work) } // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`

// Closed discards the method value and reports the group closed.
func (s *savedValue) Closed() error {
	_ = s.wg.Wait
	return errClosed
}

// panicHanded hands its group to panic, which never waits.
type panicHanded struct{ wg sync.WaitGroup }

// Start runs the worker.
func (p *panicHanded) Start() { p.wg.Go(work) } // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`

// Run panics with the group.
func (p *panicHanded) Run() { panic(&p.wg) }

// appendHanded hands its group to append, which never waits.
type appendHanded struct {
	wg  sync.WaitGroup
	all []*sync.WaitGroup
}

// Start runs the worker.
func (a *appendHanded) Start() { a.wg.Go(work) } // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`

// Closed records the group and reports the group closed.
func (a *appendHanded) Closed() error {
	a.all = append(a.all, &a.wg)
	return errClosed
}

// replacedWaitSpin replaces its waiting literal before the loop that calls
// it forever.
type replacedWaitSpin struct{ wg sync.WaitGroup }

// Start runs the worker.
func (r *replacedWaitSpin) Start() {
	r.wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// Run calls only the replacement, forever.
func (r *replacedWaitSpin) Run() {
	wait := func() { r.wg.Wait() }
	wait = func() {}
	for {
		wait()
	}
}
