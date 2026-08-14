// The TS-C05 failure modes: a blocking select with no shutdown case, and
// blocking channel operations outside any select.
package fixture

import "context"

func awaitBlocking(ready chan struct{}) {
	select { // want `TS-C05: blocking select has no shutdown case`
	case <-ready:
	}
}

func sendBlocking(results chan int) {
	results <- 1 // want `TS-C05: blocking channel operation outside a select`
}

func receiveBlocking(values chan int) int {
	v := <-values // want `TS-C05: blocking channel operation outside a select`
	return v
}

var _ = context.Background
