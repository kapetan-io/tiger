// A context built inside the function is never cancelled by a caller, so
// waiting for it to end proves nothing.
package fixture

import "context"

// untilBackgroundEnds waits on a context nobody can cancel.
func untilBackgroundEnds(frame func()) {
	ctx := context.Background()
	for ctx.Err() == nil { // want `TS-S02: tiger can't tell how many times this loop runs`
		frame()
	}
}
