// Documented TS-C02 coverage gaps.
package fixture

import "sync"

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

// waitBeforeGo waits before the goroutine starts, so the Wait joins nothing.
//
// known-miss: a Wait anywhere in the function counts; its order against the
// start is not checked.
func waitBeforeGo() {
	var wg sync.WaitGroup
	wg.Wait()
	wg.Go(work)
}
