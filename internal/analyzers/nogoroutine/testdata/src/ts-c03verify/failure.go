// A test that calls goleak.VerifyNone is not a TestMain leak check: it covers
// one test and the package's other tests can still leak.
package fixture

import "sync"

func run() {
	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {}) // want `TS-C03: this package starts goroutines but its tests have no goleak check`
}
