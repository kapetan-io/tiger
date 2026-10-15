// More TS-C05 compliant shapes: contexts held in fields or locals, package
// channels an exported function closes, promoted and generic owners, a select
// inside a function literal, and the handshake that sends before it waits.
package fixture

import "context"

// Session keeps the context its caller gave it.
type Session struct {
	ctx context.Context
}

func (s *Session) await(values chan int) int {
	select {
	case v := <-values:
		return v
	case <-s.ctx.Done():
		return 0
	}
}

func awaitDoneKept(ctx context.Context, values chan int) int {
	done := ctx.Done()
	select {
	case v := <-values:
		return v
	case <-done:
		return 0
	}
}

func awaitFromSource(source func() context.Context, values chan int) int {
	ctx := source()
	select {
	case v := <-values:
		return v
	case <-ctx.Done():
		return 0
	}
}

func waiterFor(values chan int) func(ctx context.Context) int {
	return func(ctx context.Context) int {
		select {
		case v := <-values:
			return v
		case <-ctx.Done():
			return 0
		}
	}
}

func awaitInGoroutineLiteral(ctx context.Context, values chan int) {
	go func() {
		select {
		case <-values:
		case <-ctx.Done():
		}
	}()
}

var quit = make(chan struct{})

// Quit releases awaitQuit.
func Quit() { close(quit) }

func awaitQuit(values chan int) {
	select {
	case <-quit:
	case <-values:
	}
}

// Base owns a stop channel; Embedding waits on it through promotion.
type Base struct {
	stop chan struct{}
}

// Stop releases waiters.
func (b *Base) Stop() { close(b.stop) }

// Embedding waits on the promoted channel.
type Embedding struct {
	Base
}

// Wait blocks until Stop.
func (e *Embedding) Wait(values chan int) {
	select {
	case <-e.stop:
	case <-values:
	}
}

// Lane is a generic owner that closes its stop channel in Close.
type Lane[T any] struct {
	stop   chan struct{}
	values chan T
}

// Close releases waiters.
func (l *Lane[T]) Close() { close(l.stop) }

// Wait blocks until Close.
func (l *Lane[T]) Wait() {
	select {
	case <-l.stop:
	case <-l.values:
	}
}

// Handshake sends a reply channel and waits for the loop to close it.
type Handshake struct {
	shutdownCh chan chan struct{}
}

// Loop answers a shutdown request by closing the reply.
func (h *Handshake) Loop(ctx context.Context) {
	select {
	case ready := <-h.shutdownCh:
		close(ready)
	case <-ctx.Done():
	}
}

// Shutdown sends the request and waits for the loop to close ready.
func (h *Handshake) Shutdown(ctx context.Context) error {
	ready := make(chan struct{})
	select {
	case h.shutdownCh <- ready:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Helped closes its channel through a helper that takes it as a parameter.
type Helped struct {
	stop chan struct{}
}

func shut(done chan struct{}) { close(done) }

// Close releases waiters through shut.
func (h *Helped) Close() { shut(h.stop) }

// Wait blocks until Close.
func (h *Helped) Wait(values chan int) {
	select {
	case <-h.stop:
	case <-values:
	}
}
