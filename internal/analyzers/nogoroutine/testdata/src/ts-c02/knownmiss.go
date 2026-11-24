// Documented TS-C02 coverage gaps.
package fixture

import (
	"context"
	"sync"
)

func startOnParameter(wg *sync.WaitGroup) { wg.Go(work) }

// callerNeverWaits passes its group to a function that starts a goroutine on
// it and never waits.
//
// known-miss: a group received as a parameter is the caller's to wait on, and
// this check does not follow the argument back to see that the caller does
// not.
func callerNeverWaits() {
	var wg sync.WaitGroup
	startOnParameter(&wg)
}

// waitOnEarlyPath waits only on a path that returns before the start, then
// gives up through a ctx.Done() case after it.
//
// known-miss: any Wait on the group in the function lets a ctx.Done() case
// after the start stand in for one, even a Wait the path never reaches.
func waitOnEarlyPath(ctx context.Context, early bool) {
	var wg sync.WaitGroup
	if early {
		wg.Wait()
		return
	}
	wg.Go(work)
	select {
	case <-ctx.Done():
		_ = 1 // an empty case body is not tracked as a ctx.Done() case
	}
}
