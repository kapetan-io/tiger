// The TS-S03 compliant shapes: an unbounded for{} whose select has a case that
// can stop the loop, either a receive from a context that something outside
// the function cancels, or a receive or send on a channel the package closes
// or sends on from code its exported surface reaches.
package fixture

import "context"

// runWithTermination selects forever on input, but the ctx.Done() case on a
// context the caller owns gives the loop a way to stop.
func runWithTermination(ctx context.Context, input <-chan int, output chan<- int) {
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value * 2
		}
	}
}

// runWithDerived stops on a context derived from the caller's, which the
// caller cancels through the parent.
func runWithDerived(parent context.Context, input <-chan int, output chan<- int) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value * 2
		}
	}
}

// drainPending selects with a default clause, so it never blocks: a drain
// loop bounded by the channel's contents at entry needs no stop case.
func drainPending(pending chan int) int {
	total := 0
	for {
		select {
		case value := <-pending:
			total += value
		default:
			return total
		}
	}
}

// Pump stops when Stop closes its stop channel.
type Pump struct {
	stop  chan struct{}
	input chan int
}

// Stop ends the loop run by Start. The close is in an exported method.
func (p *Pump) Stop() { close(p.stop) }

// Start runs the loop on the calling goroutine.
func (p *Pump) Start(output chan<- int) {
	for {
		select {
		case <-p.stop:
			return
		case value := <-p.input:
			output <- value * 2
		}
	}
}

// request is a payload a shutdown channel can carry, unlike a bare
// struct{} signal.
type request struct {
	id int
}

// Server stops when Shutdown sends it a request, the acknowledged-drain
// contract a request-carrying shutdown channel gives.
type Server struct {
	shutdownCh chan request
	input      chan int
}

// Shutdown hands the loop a request. The send is in an exported method.
func (s *Server) Shutdown() { s.shutdownCh <- request{id: 1} }

// Serve runs the loop on the calling goroutine.
func (s *Server) Serve(output chan<- int) {
	for {
		select {
		case req := <-s.shutdownCh:
			req.id = 0
			return
		case value := <-s.input:
			output <- value * 2
		}
	}
}

// Relay stops through a channel a helper that Close reaches closes.
type Relay struct {
	quit  chan struct{}
	input chan int
}

// Close ends the loop through an unexported helper.
func (r *Relay) Close() { r.finish() }

func (r *Relay) finish() { close(r.quit) }

// Run runs the loop on the calling goroutine.
func (r *Relay) Run(output chan<- int) {
	for {
		select {
		case <-r.quit:
			return
		case value := <-r.input:
			output <- value * 2
		}
	}
}

// RunLocal owns its stop channel and closes it itself.
func RunLocal(input <-chan int, output chan<- int, limit int) {
	stop := make(chan struct{})
	received := 0
	for {
		select {
		case <-stop:
			return
		case value := <-input:
			output <- value * 2
			received++
			if received == limit {
				close(stop)
			}
		}
	}
}

// runPassed stops on a channel its caller passes in, and the caller closes it.
func runPassed(stop <-chan struct{}, input <-chan int, output chan<- int) {
	for {
		select {
		case <-stop:
			return
		case value := <-input:
			output <- value * 2
		}
	}
}

// RunPassedAndClose passes a channel it closes to runPassed.
func RunPassedAndClose(input <-chan int, output chan<- int) {
	stop := make(chan struct{})
	close(stop)
	runPassed(stop, input, output)
}
