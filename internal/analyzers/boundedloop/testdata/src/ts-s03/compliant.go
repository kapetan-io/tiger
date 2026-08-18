// The TS-S03 compliant shape: an unbounded for{} whose select has a case
// receiving from ctx.Done(), so the loop has an explicit termination path.
package fixture

import "context"

// runWithTermination selects forever on input, but the ctx.Done() case
// gives the loop a way to stop.
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

// drainPending selects with a default clause, so it never blocks: a drain
// loop bounded by the channel's contents at entry needs no shutdown case.
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

// runWithStructSignal is bounded via the second recognized shutdown
// shape: stopCh's element type is struct{} — a pure signal, the
// closed-channel broadcast pattern — so a receive from it is a
// recognized termination path even though it is never a .Done() call.
func runWithStructSignal(stopCh chan struct{}, input <-chan int, output chan<- int) {
	for {
		select {
		case <-stopCh:
			return
		case value := <-input:
			output <- value * 2
		}
	}
}

// request is a payload a shutdown channel can carry, unlike a bare
// struct{} signal.
type request struct {
	id int
}

// runWithNamedShutdown is bounded via the name-based fallback: shutdownCh
// carries a request payload, not a struct{} signal, but its name contains
// "shutdown", so a receive from it is still a recognized termination
// path — the acknowledged-drain contract a request-carrying shutdown
// channel gives is stronger than a bare close.
func runWithNamedShutdown(shutdownCh chan request, input <-chan int, output chan<- int) {
	for {
		select {
		case req := <-shutdownCh:
			req.id = 0
			return
		case value := <-input:
			output <- value * 2
		}
	}
}
