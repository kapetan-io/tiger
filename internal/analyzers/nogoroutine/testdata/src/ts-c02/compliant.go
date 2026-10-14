// The compliant shapes: every goroutine starts through errgroup.Group.Go or
// sync.WaitGroup.Go, and a Wait on the group is reached.
package fixture

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

func work() {}

func viaErrgroup() error {
	var g errgroup.Group
	g.Go(func() error { work(); return nil })
	return g.Wait()
}

func viaWaitGroup() {
	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(work)
}

func viaWithContext(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error { work(); return nil })
	return g.Wait()
}

// startOn takes the group from its caller, who owns the Wait.
func startOn(wg *sync.WaitGroup) {
	wg.Go(work)
}

// Daemon starts its goroutine with wg.Go and waits in Shutdown.
type Daemon struct {
	wg   sync.WaitGroup
	stop chan struct{}
}

// Start runs the server goroutine.
func (d *Daemon) Start() {
	d.stop = make(chan struct{})
	d.wg.Go(func() { <-d.stop })
}

// Shutdown signals the goroutine and waits for it.
func (d *Daemon) Shutdown() {
	close(d.stop)
	d.wg.Wait()
}

// Service waits through a helper its exported Close reaches.
type Service struct {
	group errgroup.Group
}

// Start runs the worker.
func (s *Service) Start() {
	s.group.Go(func() error { work(); return nil })
}

// Close waits for the worker.
func (s *Service) Close() error { return s.join() }

func (s *Service) join() error { return s.group.Wait() }

// Loop waits on done, or gives up when the caller's ctx expires. Only the
// success path has to reach Wait.
type Loop struct {
	wg   sync.WaitGroup
	stop chan struct{}
	done chan struct{}
}

// Start runs the loop.
func (l *Loop) Start() {
	l.wg.Go(func() { <-l.stop; close(l.done) })
}

// Shutdown starts the stop and waits for the loop, or for ctx.
func (l *Loop) Shutdown(ctx context.Context) error {
	close(l.stop)
	select {
	case <-l.done:
		l.wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
