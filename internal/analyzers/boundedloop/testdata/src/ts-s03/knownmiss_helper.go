// A context a same-package helper builds from Background still looks live.
package fixture

import "context"

func ctxHelper() context.Context { return context.Background() }

// runOnHelperContext waits on a context that only Background provides.
//
// known-miss: the rule traces a context to Background within the one function,
// and ctxHelper's body is another function, so the context looks like one the
// caller could cancel and the loop that never stops goes unreported. A
// struct-field context is accepted for the same reason.
func runOnHelperContext(work <-chan int) {
	ctx := ctxHelper()
	for {
		select {
		case <-ctx.Done():
			return
		case <-work:
		}
	}
}
