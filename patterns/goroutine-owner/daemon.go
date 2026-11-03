package goroutineowner

import (
	"sync"
	"sync/atomic"
)

type Daemon struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

func (d *Daemon) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Go(func() {
		<-d.stop
		windDown()
		d.finished.Store(true)
	})
}

func (d *Daemon) Shutdown(flush func() error) error {
	close(d.stop)
	d.wg.Wait()
	return flush()
}

func (d *Daemon) Finished() bool { return d.finished.Load() }

type DaemonSkipsWait struct {
	wg       sync.WaitGroup
	stop     chan struct{}
	finished atomic.Bool
}

func (d *DaemonSkipsWait) Start(windDown func()) {
	d.stop = make(chan struct{})
	d.wg.Go(func() {
		<-d.stop
		windDown()
		d.finished.Store(true)
	})
}

func (d *DaemonSkipsWait) Shutdown(flush func() error) error {
	close(d.stop)
	if err := flush(); err != nil {
		return err
	}
	d.wg.Wait()
	return nil
}

func (d *DaemonSkipsWait) Finished() bool { return d.finished.Load() }
