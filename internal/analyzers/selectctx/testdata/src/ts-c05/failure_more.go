// More TS-C05 failure modes: contexts that never fire, selects nothing can
// release, waits on a channel the package only sends on, and the handshake
// that waits on a reply with no way out.
package fixture

import (
	"context"
	"time"
)

func awaitWithoutCancel(parent context.Context, values chan int) {
	ctx := context.WithoutCancel(parent)
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-ctx.Done():
	case <-values:
	}
}

func awaitBackgroundDone(values chan int) {
	ctx := context.Background()
	done := ctx.Done()
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-done:
	case <-values:
	}
}

func awaitTimer(values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-time.After(time.Second):
	case <-values:
	}
}

func blockForever() {
	select {} // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
}

// AwaitParameter receives a channel no code in the package closes.
func AwaitParameter(stop chan struct{}, values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-stop:
	case <-values:
	}
}

// emit is the only code that sends on out, and the send is the case itself.
func emit(out chan int, value int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case out <- value:
	}
}

// Ticket is fulfilled by a send, so a bare receive waits for a reply.
type Ticket struct {
	reply chan int
}

// Fulfil sends the reply, or gives up when ctx does.
func (t *Ticket) Fulfil(ctx context.Context, value int) {
	select {
	case t.reply <- value:
	case <-ctx.Done():
	}
}

// Redeem waits for a reply that may never come.
func (t *Ticket) Redeem() int {
	return <-t.reply // want `TS-C05: this channel operation blocks outside a select`
}

// Cell owns the stop channel.
type Cell struct {
	stop chan struct{}
}

// Warden closes a channel that belongs to Cell, not to Warden.
type Warden struct {
	cell *Cell
}

// Release closes another type's channel.
func (w *Warden) Release() { close(w.cell.stop) }

// Wait blocks on a channel only another type closes.
func (c *Cell) Wait(values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-c.stop:
	case <-values:
	}
}

var halt = make(chan struct{})

func abandonHalt() { close(halt) }

// AwaitHalt waits on a package channel only unreachable code closes.
func AwaitHalt(values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-halt:
	case <-values:
	}
}

// Courier has the call 7 handshake: Shutdown sends ready to the loop and
// waits on it with no way out.
type Courier struct {
	shutdownCh chan chan struct{}
}

// Loop answers a shutdown request by closing the reply.
func (c *Courier) Loop(ctx context.Context) {
	select {
	case ready := <-c.shutdownCh:
		close(ready)
	case <-ctx.Done():
	}
}

// Shutdown waits for the loop's answer, and ctx bounds only the send.
func (c *Courier) Shutdown(ctx context.Context) error {
	ready := make(chan struct{})
	select {
	case c.shutdownCh <- ready:
		<-ready // want `TS-C05: this channel operation blocks outside a select`
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
