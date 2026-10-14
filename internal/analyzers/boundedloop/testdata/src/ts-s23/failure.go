// The TS-S23 shapes: an event loop that has a stop case must leave only
// through it. Any other return, or a break that ends the loop, skips whatever
// the stop case does on the way out.
package fixture

import (
	"context"
	"sync"
	"sync/atomic"
)

// Pipeline stops when Stop closes its stop channel.
type Pipeline struct {
	stop     chan struct{}
	done     chan struct{}
	requests chan func()
	closing  bool
}

// Stop ends the loop.
func (p *Pipeline) Stop() { close(p.stop) }

// Serve returns from a request case when the pipeline is closing, skipping the
// stop case that closes done.
func (p *Pipeline) Serve() {
	for {
		select {
		case work := <-p.requests:
			work()
			if p.closing {
				return // want `TS-S23: this return leaves the event loop without its stop case`
			}
		case <-p.stop:
			close(p.done)
			return
		}
	}
}

// ServeAfter returns after the select instead of from a case.
func (p *Pipeline) ServeAfter() {
	for {
		select {
		case work := <-p.requests:
			work()
		case <-p.stop:
			close(p.done)
			return
		}
		if p.closing {
			return // want `TS-S23: this return leaves the event loop without its stop case`
		}
	}
}

// ServeBreak breaks out of the loop through a labeled break.
func (p *Pipeline) ServeBreak() {
outer:
	for {
		select {
		case work := <-p.requests:
			work()
			if p.closing {
				break outer // want `TS-S23: this break leaves the event loop without its stop case`
			}
		case <-p.stop:
			return
		}
	}
	close(p.done)
}

// EarlyExitLoop has the shape behind querator's ENG-160: its requests channel
// is sent on by Submit and its shutdown channel by Shutdown, so both cases look
// like stop cases. The request case returns on a flag, skipping the shutdown
// case that does the cleanup, and only the case that ends in a return counts
// as the stop case.
type EarlyExitLoop struct {
	requests   chan func()
	shutdownCh chan chan struct{}
	inShutdown atomic.Bool
	cleaned    atomic.Bool
	wg         sync.WaitGroup
}

func (l *EarlyExitLoop) run() {
	for {
		select {
		case work := <-l.requests:
			work()
			if l.inShutdown.Load() {
				return // want `TS-S23: this return leaves the event loop without its stop case`
			}
		case ready := <-l.shutdownCh:
			l.cleaned.Store(true)
			close(ready)
			return
		}
	}
}

// Start runs the loop.
func (l *EarlyExitLoop) Start() { l.wg.Go(l.run) }

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
