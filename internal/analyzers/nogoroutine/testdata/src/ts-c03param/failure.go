// A start on a group received as a parameter passes TS-C02, but it is still a
// goroutine start for TS-C03.
package fixture

import "sync"

func startOn(wg *sync.WaitGroup) {
	wg.Go(func() {}) // want `TS-C03: this package starts goroutines but its tests have no goleak check`
}
