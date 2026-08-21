// The compliant rewrites: a blocking select carries a ctx.Done() case or a
// recognized shutdown-channel case (element type struct{}, or a shutdown
// name), a select with a default clause is non-blocking so it needs no
// shutdown case at all, and a bare receive or send is exempt only when the
// channel is shutdown-recognized by NAME — same as a bare <-ctx.Done().
package fixture

import "context"

func awaitWithShutdown(ctx context.Context, ready chan struct{}) error {
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func pollNonBlocking(ready chan struct{}) bool {
	select {
	case <-ready:
		return true
	default:
		return false
	}
}

func sendWithShutdown(ctx context.Context, results chan int, value int) error {
	select {
	case results <- value:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func receiveAssignWithShutdown(ctx context.Context, values chan int) (int, error) {
	select {
	case v := <-values:
		return v, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func awaitShutdown(ctx context.Context) {
	// The bare receive from Done() is itself the shutdown wait.
	<-ctx.Done()
}

// A struct{}-element channel is the closed-channel broadcast shape: the
// single case receiving from it is itself the shutdown handoff, so the
// select needs no separate ctx.Done() case.
func awaitStructShutdown(ready chan struct{}) {
	select {
	case <-ready:
	}
}

// stopCh is a plain chan int, but its name carries the "stop" token, so
// the name-based fallback recognizes it as a shutdown channel — the
// request-carrying shutdown channel shape.
func awaitNamedShutdown(stopCh chan int) {
	select {
	case <-stopCh:
	}
}

// A bare receive from a recognized shutdown channel is itself the
// shutdown wait, the same exemption a bare <-ctx.Done() gets.
func awaitBareShutdown(stopCh chan int) {
	<-stopCh
}

// A bare send to a recognized shutdown channel is the shutdown handoff
// itself, so it needs no select wrapping it.
func signalDone(done chan struct{}) {
	done <- struct{}{}
}
