// A loop that runs until the caller cancels a context parameter is a forever
// loop, like a select on ctx.Done().
package fixture

import "context"

// untilCancelled draws frames until the caller cancels ctx.
func untilCancelled(ctx context.Context, frame func()) {
	for ctx.Err() == nil {
		frame()
	}
}

// untilCancelledOrDone adds a second way out.
func untilCancelledOrDone(ctx context.Context, done func() bool) {
	for ctx.Err() == nil && !done() {
	}
}
