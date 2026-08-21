// The TS-C05 failure modes: a blocking select with no shutdown case, and
// blocking channel operations outside any select. Each uses an
// unrecognized data channel — plain chan int with a neutral name — so
// neither the struct{}-element nor the shutdown-name recognition silences
// the finding.
package fixture

import "context"

func awaitBlocking(values chan int) {
	select { // want `TS-C05: blocking select has no shutdown case`
	case <-values:
	}
}

func sendBlocking(results chan int) {
	results <- 1 // want `TS-C05: blocking channel operation outside a select`
}

func receiveBlocking(values chan int) int {
	v := <-values // want `TS-C05: blocking channel operation outside a select`
	return v
}

// A struct{} element type never exempts a BARE operation: outside a
// select, a pure-signal channel with a neutral name is a completion wait
// (querator's req.ReadyCh) — the missing-cancellation bug itself. Only
// the name recognition, or a real select, silences a bare op.
func awaitCompletion(readyCh chan struct{}) {
	<-readyCh // want `TS-C05: blocking channel operation outside a select`
}

func signalCompletion(readyCh chan struct{}) {
	readyCh <- struct{}{} // want `TS-C05: blocking channel operation outside a select`
}

var _ = context.Background
