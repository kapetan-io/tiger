// More TS-S23 compliant shapes: breaks that leave only the select, switch or
// inner loop they sit in, and a stop case on a context.
package fixture

import "context"

// ServeBreakInRequestCase breaks out of the select, not the loop.
func (p *Pipeline) ServeBreakInRequestCase() {
	for {
		select {
		case work := <-p.requests:
			if p.closing {
				break
			}
			work()
		case <-p.stop:
			return
		}
	}
}

// ServeSwitchBreak breaks out of a switch.
func (p *Pipeline) ServeSwitchBreak() {
	for {
		select {
		case work := <-p.requests:
			switch {
			case p.closing:
				break
			default:
				work()
			}
		case <-p.stop:
			return
		}
	}
}

// ServeInnerLoop breaks out of loops declared inside the case.
func (p *Pipeline) ServeInnerLoop() {
	for {
		select {
		case work := <-p.requests:
		inner:
			for range 3 {
				for range 3 {
					break inner
				}
				break
			}
			work()
		case <-p.stop:
			return
		}
	}
}

func serveContext(ctx context.Context, requests <-chan func(), done chan<- struct{}) {
	for {
		select {
		case work := <-requests:
			work()
		case <-ctx.Done():
			if done == nil {
				return
			}
			close(done)
			return
		}
	}
}
