// The TS-S03 failure mode: an unbounded for{} whose select has no case
// receiving from ctx.Done(), so it has no way to be told to stop.
package fixture

// runNoTermination selects forever on input and mirrors it to output, but
// nothing in the select can end the loop.
func runNoTermination(input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this unbounded event loop's select has no case`
		case value := <-input:
			output <- value * 2
		}
	}
}
