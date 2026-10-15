// A case that receives from a live ctx.Done() or a closed channel but does not
// leave the loop does not stop it.
package fixture

import "context"

func runIgnoresContext(ctx context.Context, work <-chan int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-ctx.Done():
		case <-work:
		}
	}
}

// Sleeper closes its stop channel and then ignores it.
type Sleeper struct {
	stop chan struct{}
	work chan int
}

// Close closes the stop channel.
func (s *Sleeper) Close() { close(s.stop) }

// Run reads stop and carries on.
func (s *Sleeper) Run() {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-s.stop:
			continue
		case <-s.work:
		}
	}
}
