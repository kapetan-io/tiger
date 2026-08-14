// The compliant rewrites: a blocking select carries a ctx.Done() case, and
// a select with a default clause is non-blocking so it needs no shutdown
// case at all.
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
