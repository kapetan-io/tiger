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
