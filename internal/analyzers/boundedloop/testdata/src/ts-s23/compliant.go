// The TS-S23 compliant shapes: an event loop that leaves only through its stop
// case. A return inside a function literal, a break out of an inner select, and
// a continue do not leave the loop.
package fixture

import (
	"context"
	"sync"
	"sync/atomic"
)

// ServeOnlyStop leaves only from its stop case, the shape to copy.
func (p *Pipeline) ServeOnlyStop() {
	for {
		select {
		case work := <-p.requests:
			work()
			if p.closing {
				continue
			}
		case <-p.stop:
			close(p.done)
			return
		}
	}
}

// ServeClosure returns from a function literal, which leaves the literal and
// not the loop.
func (p *Pipeline) ServeClosure() {
	for {
		select {
		case work := <-p.requests:
			func() {
				if p.closing {
					return
				}
				work()
			}()
		case <-p.stop:
			return
		}
	}
}

// ServeInnerBreak breaks out of an inner select, which leaves the select and
// not the loop.
func (p *Pipeline) ServeInnerBreak() {
	for {
		select {
		case work := <-p.requests:
			select {
			case <-p.done:
				break
			default:
				work()
			}
		case <-p.stop:
			return
		}
	}
}

// StopDoneLoop has the shape net/http's Server.Shutdown uses: close stop once,
// the loop's only exit is its stop case, the loop closes done, and ctx bounds
// only the caller's wait.
type StopDoneLoop struct {
	requests   chan func()
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
	inShutdown atomic.Bool
	cleaned    atomic.Bool
	wg         sync.WaitGroup
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

// Start runs the loop.
func (l *StopDoneLoop) Start() { l.wg.Go(l.run) }

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
