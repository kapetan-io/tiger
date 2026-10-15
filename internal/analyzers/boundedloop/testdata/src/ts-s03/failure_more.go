// More TS-S03 failure modes: contexts that never fire, channels the package
// never closes, and closes that the exported surface cannot make.
package fixture

import (
	"context"
	"time"
)

// runWithoutCancel waits on a context that WithoutCancel detached from its
// parent, whose Done channel is nil.
func runWithoutCancel(parent context.Context, input <-chan int, output chan<- int) {
	ctx := context.WithoutCancel(parent)
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}

func runOnBackgroundDone(input <-chan int, output chan<- int) {
	ctx := context.Background()
	done := ctx.Done()
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-done:
			return
		case value := <-input:
			output <- value
		}
	}
}

func runOnTimer(input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-time.After(time.Second):
			return
		case value := <-input:
			output <- value
		}
	}
}

// RunOnParameter receives a channel no code in the package closes.
func RunOnParameter(stop <-chan struct{}, input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-stop:
			return
		case value := <-input:
			output <- value
		}
	}
}

func runPassedNeverClosed(stop <-chan struct{}, input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-stop:
			return
		case value := <-input:
			output <- value
		}
	}
}

// StartPassedNeverClosed passes runPassedNeverClosed a channel it never closes.
func StartPassedNeverClosed(input <-chan int, output chan<- int) {
	stop := make(chan struct{})
	runPassedNeverClosed(stop, input, output)
}

// Cell owns the stop channel.
type Cell struct {
	stop  chan struct{}
	input chan int
}

// Warden closes a channel that belongs to Cell, not to Warden.
type Warden struct {
	cell *Cell
}

// Release closes another type's channel.
func (w *Warden) Release() { close(w.cell.stop) }

// Run runs the loop on the calling goroutine.
func (c *Cell) Run(output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-c.stop:
			return
		case value := <-c.input:
			output <- value
		}
	}
}

var halt = make(chan struct{})

// abandonHalt closes halt, but nothing exported reaches it.
func abandonHalt() { close(halt) }

// RunUntilHalt waits on a package channel only dead code closes.
func RunUntilHalt(input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-halt:
			return
		case value := <-input:
			output <- value
		}
	}
}

var drainStop = make(chan struct{})

// RunUntilDrain waits on a package channel only a test closes.
func RunUntilDrain(input <-chan int, output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-drainStop:
			return
		case value := <-input:
			output <- value
		}
	}
}

// Sender sends on its channel in a method nothing exported reaches.
type Sender struct {
	signal chan int
	input  chan int
}

func (s *Sender) poke() { s.signal <- 1 }

// Run runs the loop on the calling goroutine.
func (s *Sender) Run(output chan<- int) {
	for {
		select { // want `TS-S03: this event loop's select has no case that stops the loop`
		case <-s.signal:
			return
		case value := <-s.input:
			output <- value
		}
	}
}
