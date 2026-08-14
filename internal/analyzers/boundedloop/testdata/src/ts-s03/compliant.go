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
