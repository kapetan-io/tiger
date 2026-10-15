// The TS-S03 failure modes: an unbounded for{} whose select has no case that
// can stop it. A case counts only when something can fire it, so a method
// named Done, a channel with a shutdown-sounding name, and a channel with a
// struct{} element all fail when nothing closes them.
package fixture

import "context"

// runNoTermination selects forever on input and mirrors it to output, but
// nothing in the select can end the loop.
func runNoTermination(input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case value := <-input:
			output <- value * 2
		}
	}
}

// Latch has a Done method that never fires.
type Latch struct{}

// Done returns a nil channel, so a receive on it blocks forever.
func (Latch) Done() <-chan bool { return nil }

// drainLatch sums work until the latch fires, which it never does.
func drainLatch(latch Latch, work <-chan int) int {
	total := 0
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-latch.Done():
			return total
		case n := <-work:
			total += n
		}
	}
}

// Worker selects on a channel named shutdown that nothing ever closes or
// sends on.
type Worker struct {
	shutdown chan bool
	work     chan int
}

// Run sums work until shutdown, which never comes.
func (w *Worker) Run() int {
	total := 0
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-w.shutdown:
			return total
		case n := <-w.work:
			total += n
		}
	}
}

// runSignalNeverClosed receives from a struct{} channel nothing closes.
func runSignalNeverClosed(stopCh chan struct{}, input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-stopCh:
			return
		case value := <-input:
			output <- value * 2
		}
	}
}

// runOnBackground waits on a context made from Background in this function,
// which only this function can cancel.
func runOnBackground(input <-chan int, output chan<- int) {
	ctx := context.Background()
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value * 2
		}
	}
}

// runOnDerivedBackground derives from Background, which is still a context
// nothing outside the function cancels.
func runOnDerivedBackground(input <-chan int, output chan<- int) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value * 2
		}
	}
}

// Orphan closes its stop channel in a method no exported method reaches.
type Orphan struct {
	stop  chan struct{}
	input chan int
}

func (o *Orphan) abandon() { close(o.stop) }

// Run runs the loop on the calling goroutine.
func (o *Orphan) Run(output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-o.stop:
			return
		case value := <-o.input:
			output <- value * 2
		}
	}
}
