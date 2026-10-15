// The TS-C05 failure modes: a blocking select with no stop case, and
// blocking channel operations outside any select. A case counts only when
// something can fire it, so a plain data channel, a channel with a
// shutdown-sounding name that nothing closes, a struct{} channel nobody
// closes, and a method named Done that never fires all fail.
package fixture

import "context"

func awaitBlocking(values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-values:
	}
}

func sendBlocking(results chan int) {
	results <- 1 // want `TS-C05: this channel operation blocks outside a select`
}

func receiveBlocking(values chan int) int {
	v := <-values // want `TS-C05: this channel operation blocks outside a select`
	return v
}

// A struct{} element type never exempts a BARE operation: outside a
// select, a pure-signal channel nothing closes is a completion wait
// (querator's req.ReadyCh) — the missing-cancellation bug itself.
func awaitCompletion(readyCh chan struct{}) {
	<-readyCh // want `TS-C05: this channel operation blocks outside a select`
}

func signalCompletion(readyCh chan struct{}) {
	readyCh <- struct{}{} // want `TS-C05: this channel operation blocks outside a select`
}

// Latch has a Done method that never fires.
type Latch struct{}

// Done returns a nil channel, so a receive on it blocks forever.
func (Latch) Done() <-chan bool { return nil }

func awaitLatch(latch Latch, values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-latch.Done():
	case <-values:
	}
}

// Worker waits on a channel named shutdown that nothing closes or sends on.
type Worker struct {
	shutdown chan bool
}

func (w *Worker) await(values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-w.shutdown:
	case <-values:
	}
}

// awaitNamedStop names its channel like a stop signal, and nothing closes it.
func awaitNamedStop(stopCh chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-stopCh:
	}
}

func awaitNamedBare(doneCh chan struct{}) {
	<-doneCh // want `TS-C05: this channel operation blocks outside a select`
}

func signalDone(done chan struct{}) {
	done <- struct{}{} // want `TS-C05: this channel operation blocks outside a select`
}

// awaitOnBackground waits on a context made from Background in this
// function, which only this function can cancel.
func awaitOnBackground(values chan int) {
	ctx := context.Background()
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-ctx.Done():
	case <-values:
	}
}

// Orphan closes its channel in a method no exported method reaches.
type Orphan struct {
	stop chan struct{}
}

func (o *Orphan) abandon() { close(o.stop) }

// Wait blocks on stop.
func (o *Orphan) Wait(values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-o.stop:
	case <-values:
	}
}
