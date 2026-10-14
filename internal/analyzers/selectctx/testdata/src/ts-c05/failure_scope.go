// A context that never cancels is not a stop case, however it is built.
package fixture

import "context"

func awaitDetachedLocal(ctx context.Context, values chan int) {
	select { // want `TS-C05: this select blocks with no case that ends the wait on shutdown`
	case <-context.WithoutCancel(ctx).Done():
	case <-values:
	}
}
