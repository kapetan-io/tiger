// More TS-S23 failure modes: exits beside a context stop case, in nested
// statements, after the select, through a timer case, and by goto.
package fixture

import (
	"context"
	"time"
)

func serveContextEarlyReturn(ctx context.Context, requests <-chan func(), done chan<- struct{}) {
	for {
		select {
		case work := <-requests:
			if work == nil {
				return // want `TS-S23: this return leaves the event loop without its stop case`
			}
			work()
		case <-ctx.Done():
			close(done)
			return
		}
	}
}

// ServeNestedReturn returns from inside a loop in the request case.
func (p *Pipeline) ServeNestedReturn() {
	for {
		select {
		case work := <-p.requests:
			for range 2 {
				if p.closing {
					return // want `TS-S23: this return leaves the event loop without its stop case`
				}
			}
			work()
		case <-p.stop:
			close(p.done)
			return
		}
	}
}

// ServeBreakAfterSelect breaks from the loop body after the select.
func (p *Pipeline) ServeBreakAfterSelect() {
	for {
		select {
		case work := <-p.requests:
			work()
		case <-p.stop:
			close(p.done)
			return
		}
		if p.closing {
			break // want `TS-S23: this break leaves the event loop without its stop case`
		}
	}
	close(p.done)
}

// ServeTimer returns from a timer case.
func (p *Pipeline) ServeTimer(idle time.Duration) {
	for {
		select {
		case work := <-p.requests:
			work()
		case <-time.After(idle):
			return // want `TS-S23: this return leaves the event loop without its stop case`
		case <-p.stop:
			close(p.done)
			return
		}
	}
}

// ServeGoto jumps out of the loop.
func (p *Pipeline) ServeGoto() {
	for {
		select {
		case work := <-p.requests:
			if p.closing {
				goto out // want `TS-S23: this \w+ leaves the event loop without its stop case`
			}
			work()
		case <-p.stop:
			return
		}
	}
out:
	close(p.done)
}
