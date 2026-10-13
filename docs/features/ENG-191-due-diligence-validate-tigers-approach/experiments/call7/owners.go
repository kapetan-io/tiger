package call7

import (
	"sync"
	"sync/atomic"
)

// UnwaitedDaemon has querator's trial shape: Add, go and Done, but Shutdown
// never waits, so it returns while the goroutine still runs.
type UnwaitedDaemon struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

// Start runs the server goroutine, which needs a moment to wind down.
func (d *UnwaitedDaemon) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		<-d.stop
		windDown()
		d.finished.Store(true)
	}()
}

// Shutdown signals the goroutine and returns without waiting for it.
func (d *UnwaitedDaemon) Shutdown() { close(d.stop) }

// Finished reports whether the goroutine has exited.
func (d *UnwaitedDaemon) Finished() bool { return d.finished.Load() }

// WaitedDaemon starts its goroutine with wg.Go and waits in Shutdown.
type WaitedDaemon struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

// Start runs the server goroutine.
func (d *WaitedDaemon) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Go(func() {
		<-d.stop
		windDown()
		d.finished.Store(true)
	})
}

// Shutdown signals the goroutine and waits for it.
func (d *WaitedDaemon) Shutdown() {
	close(d.stop)
	d.wg.Wait()
}

// Finished reports whether the goroutine has exited.
func (d *WaitedDaemon) Finished() bool { return d.finished.Load() }

// GoNoWaitDaemon uses wg.Go, which today's TS-C02 never inspects, and never
// waits: the trial bug in a form tiger accepts.
type GoNoWaitDaemon struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

// Start runs the server goroutine.
func (d *GoNoWaitDaemon) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Go(func() {
		<-d.stop
		windDown()
		d.finished.Store(true)
	})
}

// Shutdown signals the goroutine and returns without waiting.
func (d *GoNoWaitDaemon) Shutdown() { close(d.stop) }

// Finished reports whether the goroutine has exited.
func (d *GoNoWaitDaemon) Finished() bool { return d.finished.Load() }
