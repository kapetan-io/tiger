package goroutineowner

import (
	"sync"
	"sync/atomic"
)

// Daemon is the right shape: the type that starts the goroutine also waits for it.
type Daemon struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

// Start runs the server goroutine; windDown stands in for its cleanup work.
func (d *Daemon) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Go(func() {
		<-d.stop
		windDown()
		d.finished.Store(true)
	})
}

// Shutdown signals the goroutine, then waits. The wait must follow the signal or it never returns.
func (d *Daemon) Shutdown() {
	close(d.stop)
	d.wg.Wait()
}

// Finished reports whether the goroutine has exited.
func (d *Daemon) Finished() bool { return d.finished.Load() }

// DaemonNoWait is the broken shape: wg.Go is not inspected by TS-C02, so tiger
// accepts it, but Shutdown returns while the goroutine is still winding down.
type DaemonNoWait struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

// Start runs the server goroutine.
func (d *DaemonNoWait) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Go(func() {
		<-d.stop
		windDown()
		d.finished.Store(true)
	})
}

// Shutdown signals the goroutine and returns without waiting for it.
func (d *DaemonNoWait) Shutdown() { close(d.stop) }

// Finished reports whether the goroutine has exited.
func (d *DaemonNoWait) Finished() bool { return d.finished.Load() }
