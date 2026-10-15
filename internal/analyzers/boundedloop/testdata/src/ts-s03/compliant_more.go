// More TS-S03 compliant shapes: contexts held in fields or reassigned, a
// Done channel kept in a local, a channel aliased from a field, a package
// variable, embedded and generic owners.
package fixture

import (
	"context"
	"os"
	"os/signal"
)

// Session keeps the context its caller gave it.
type Session struct {
	ctx   context.Context
	input chan int
}

// Run stops when the session's context is cancelled.
func (s *Session) Run(output chan<- int) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case value := <-s.input:
			output <- value
		}
	}
}

func runWithDoneKept(ctx context.Context, input <-chan int, output chan<- int) {
	done := ctx.Done()
	for {
		select {
		case <-done:
			return
		case value := <-input:
			output <- value
		}
	}
}

func runWithCtxCopy(ctx context.Context, input <-chan int, output chan<- int) {
	held := ctx
	for {
		select {
		case <-held.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}

// runReassignedToParent starts from Background and ends on the caller's ctx.
func runReassignedToParent(parent context.Context, input <-chan int, output chan<- int) {
	ctx := context.Background()
	ctx = parent
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}

// runOnSignal stops on an interrupt, which Background cannot deliver.
func runOnSignal(input <-chan int, output chan<- int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}

type Ctx = context.Context

func runWithAliasType(ctx Ctx, input <-chan int, output chan<- int) {
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}

func runFromSource(source func() context.Context, input <-chan int, output chan<- int) {
	ctx := source()
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}

// Courier reads its stop channel into a local before the loop.
type Courier struct {
	stop  chan struct{}
	input chan int
}

// Stop ends the loop.
func (c *Courier) Stop() { close(c.stop) }

// Run stops on the aliased channel.
func (c *Courier) Run(output chan<- int) {
	stop := c.stop
	for {
		select {
		case <-stop:
			return
		case value := <-c.input:
			output <- value
		}
	}
}

var quit = make(chan struct{})

// Quit ends RunUntilQuit.
func Quit() { close(quit) }

// RunUntilQuit stops on a package-level channel an exported function closes.
func RunUntilQuit(input <-chan int, output chan<- int) {
	for {
		select {
		case <-quit:
			return
		case value := <-input:
			output <- value
		}
	}
}

// Base owns a stop channel; Embedding gets it by promotion.
type Base struct {
	stop chan struct{}
}

// Stop ends the loop.
func (b *Base) Stop() { close(b.stop) }

// Embedding runs on a channel it inherits.
type Embedding struct {
	Base
	input chan int
}

// Run stops on the promoted channel, which Base.Stop closes.
func (e *Embedding) Run(output chan<- int) {
	for {
		select {
		case <-e.stop:
			return
		case value := <-e.input:
			output <- value
		}
	}
}

// Inner loops on its own channel; Outer closes it through promotion.
type Inner struct {
	stop  chan struct{}
	input chan int
}

// Run stops on the channel the embedding type closes.
func (i *Inner) Run(output chan<- int) {
	for {
		select {
		case <-i.stop:
			return
		case value := <-i.input:
			output <- value
		}
	}
}

// Outer embeds Inner.
type Outer struct {
	Inner
}

// Close ends the loop through the promoted field.
func (o *Outer) Close() { close(o.stop) }

// Lane is a generic owner that closes its stop channel in Close.
type Lane[T any] struct {
	stop  chan struct{}
	input chan T
}

// Close ends the loop.
func (l *Lane[T]) Close() { close(l.stop) }

// Run stops on the channel Close closes.
func (l *Lane[T]) Run(output chan<- T) {
	for {
		select {
		case <-l.stop:
			return
		case value := <-l.input:
			output <- value
		}
	}
}

// Helped closes its channel through a helper that takes it as a parameter.
type Helped struct {
	stop  chan struct{}
	input chan int
}

func shut(done chan struct{}) { close(done) }

// Close ends the loop through shut.
func (h *Helped) Close() { shut(h.stop) }

// Run stops on the channel Close hands to shut.
func (h *Helped) Run(output chan<- int) {
	for {
		select {
		case <-h.stop:
			return
		case value := <-h.input:
			output <- value
		}
	}
}
