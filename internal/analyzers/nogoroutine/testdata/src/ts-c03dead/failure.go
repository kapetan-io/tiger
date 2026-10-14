// A VerifyTestMain call under `if false` never runs, so the package has no
// leak check.
package fixture

import "sync"

func run() {
	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {}) // want `TS-C03: this package starts goroutines but its tests have no goleak check`
}
