// A context a same-package helper builds from Background still looks live.
package fixture

import "context"

func ctxHelper() context.Context { return context.Background() }

// awaitOnHelperContext waits on a context that only Background provides.
//
// known-miss: the rule traces a context to Background within the one function,
// and ctxHelper's body is another function, so the select looks stoppable.
func awaitOnHelperContext(work <-chan int) {
	ctx := ctxHelper()
	select {
	case <-ctx.Done():
	case <-work:
	}
}
