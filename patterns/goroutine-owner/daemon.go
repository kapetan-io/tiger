package goroutineowner

import (
	"sync"
	"sync/atomic"
)

// Daemon is the right shape: the type that starts the goroutine also waits for
// it on every way out of Shutdown.
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

// Shutdown signals the goroutine, waits for it, and only then flushes. The
// wait must follow the signal or it never returns, and it comes before the
// flush so a flush error cannot skip it.
func (d *Daemon) Shutdown(flush func() error) error {
	close(d.stop)
	d.wg.Wait()
	return flush()
}

// Finished reports whether the goroutine has exited.
func (d *Daemon) Finished() bool { return d.finished.Load() }

// DaemonSkipsWait is the broken shape: Shutdown does wait, so TS-C02 accepts
// it, but it returns on a flush error before the wait and leaves the goroutine
// winding down.
type DaemonSkipsWait struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

// Start runs the server goroutine.
func (d *DaemonSkipsWait) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Go(func() {
		<-d.stop
		windDown()
		d.finished.Store(true)
	})
}

// Shutdown signals the goroutine, flushes, and waits only if the flush worked.
func (d *DaemonSkipsWait) Shutdown(flush func() error) error {
	close(d.stop)
	if err := flush(); err != nil {
		return err
	}
	d.wg.Wait()
	return nil
}

// Finished reports whether the goroutine has exited.
func (d *DaemonSkipsWait) Finished() bool { return d.finished.Load() }
