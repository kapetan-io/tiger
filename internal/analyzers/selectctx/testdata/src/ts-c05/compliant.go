// The compliant rewrites: a blocking select carries a ctx.Done() case on a
// context something outside the function cancels, or a case on a channel the
// package closes or sends on. A select with a default clause is non-blocking
// so it needs no stop case at all, and a bare receive is exempt only when it
// is itself the wait for a ctx.Done() or a close the package performs.
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

// Gate waits on a channel its exported Open method closes, so the select
// case is a stop case the package can fire.
type Gate struct {
	open chan struct{}
}

// Open releases everyone waiting on the gate.
func (g *Gate) Open() { close(g.open) }

// Wait blocks until Open is called.
func (g *Gate) Wait() {
	select {
	case <-g.open:
	}
}

// WaitBare blocks on the same channel outside a select. The receive is the
// wait for a close the package performs, so it is exempt like <-ctx.Done().
func (g *Gate) WaitBare() {
	<-g.open
}

// Mailbox is told to stop with a request on a channel that carries data, the
// request-carrying shutdown shape: Shutdown sends on it.
type Mailbox struct {
	shutdownCh chan int
	values     chan int
}

// Shutdown sends the stop request, or gives up when ctx does.
func (m *Mailbox) Shutdown(ctx context.Context) error {
	select {
	case m.shutdownCh <- 1:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Next waits for a value or the stop request.
func (m *Mailbox) Next() int {
	select {
	case n := <-m.shutdownCh:
		return n
	case v := <-m.values:
		return v
	}
}

// awaitLocalStop owns the channel it waits on and closes it itself.
func awaitLocalStop(values chan int) int {
	stop := make(chan struct{})
	go func() { close(stop) }()
	select {
	case <-stop:
		return 0
	case v := <-values:
		return v
	}
}

// awaitPassedStop waits on a channel its caller passes in, and the caller
// closes it.
func awaitPassedStop(stop chan struct{}, values chan int) int {
	select {
	case <-stop:
		return 0
	case v := <-values:
		return v
	}
}

// AwaitWithPassedStop passes awaitPassedStop a channel it closes.
func AwaitWithPassedStop(values chan int) int {
	stop := make(chan struct{})
	close(stop)
	return awaitPassedStop(stop, values)
}

// awaitDerived waits on a context derived from the caller's.
func awaitDerived(parent context.Context, values chan int) (int, error) {
	ctx, cancel := context.WithTimeout(parent, 1)
	defer cancel()
	select {
	case v := <-values:
		return v, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
