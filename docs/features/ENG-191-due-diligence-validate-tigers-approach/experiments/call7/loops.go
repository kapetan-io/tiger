package call7

import (
	"context"
	"sync"
	"sync/atomic"
)

// EarlyExitLoop has the shape behind querator's ENG-160: the request case
// returns on a flag, skipping the shutdown handler.
type EarlyExitLoop struct {
	requests   chan func()
	shutdownCh chan chan struct{}
	inShutdown atomic.Bool
	cleaned    atomic.Bool
	wg         sync.WaitGroup
}

// NewEarlyExitLoop starts the loop.
func NewEarlyExitLoop() *EarlyExitLoop {
	l := &EarlyExitLoop{requests: make(chan func(), 8), shutdownCh: make(chan chan struct{})}
	l.wg.Go(l.run)
	return l
}

func (l *EarlyExitLoop) run() {
	for {
		select {
		case work := <-l.requests:
			work()
			if l.inShutdown.Load() {
				return
			}
		case ready := <-l.shutdownCh:
			l.cleaned.Store(true)
			close(ready)
			return
		}
	}
}

// Submit queues work for the loop.
func (l *EarlyExitLoop) Submit(work func()) { l.requests <- work }

// Shutdown hands the loop a shutdown request and waits for it, or for ctx.
func (l *EarlyExitLoop) Shutdown(ctx context.Context) error {
	if l.inShutdown.Swap(true) {
		return nil
	}
	ready := make(chan struct{})
	select {
	case l.shutdownCh <- ready:
		<-ready
		l.wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cleaned reports whether the shutdown handler ran.
func (l *EarlyExitLoop) Cleaned() bool { return l.cleaned.Load() }

// StopDoneLoop has the shape call 7's messages prescribe, the one net/http's
// Server.Shutdown uses: close stop once, the loop's only exit is its stop
// case, the loop closes done, and ctx bounds only the caller's wait.
type StopDoneLoop struct {
	requests   chan func()
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
	inShutdown atomic.Bool
	cleaned    atomic.Bool
	wg         sync.WaitGroup
}

// NewStopDoneLoop starts the loop.
func NewStopDoneLoop() *StopDoneLoop {
	l := &StopDoneLoop{requests: make(chan func(), 8), stop: make(chan struct{}), done: make(chan struct{})}
	l.wg.Go(l.run)
	return l
}

func (l *StopDoneLoop) run() {
	l.serve()
	l.cleaned.Store(true)
	close(l.done)
}

// serve returns only from the stop case.
func (l *StopDoneLoop) serve() {
	for {
		select {
		case work := <-l.requests:
			work()
		case <-l.stop:
			return
		}
	}
}

// Submit queues work for the loop.
func (l *StopDoneLoop) Submit(work func()) { l.requests <- work }

// Shutdown starts the shutdown once and waits for it, or for ctx. A later call
// waits again, so a caller whose ctx expired can retry.
func (l *StopDoneLoop) Shutdown(ctx context.Context) error {
	l.inShutdown.Store(true)
	l.stopOnce.Do(func() { close(l.stop) })
	select {
	case <-l.done:
		l.wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cleaned reports whether the shutdown handler ran.
func (l *StopDoneLoop) Cleaned() bool { return l.cleaned.Load() }

// InShutdown reports whether Shutdown has started.
func (l *EarlyExitLoop) InShutdown() bool { return l.inShutdown.Load() }
