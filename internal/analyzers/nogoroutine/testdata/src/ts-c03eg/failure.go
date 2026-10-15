// errgroup.Group.Go starts a goroutine, so the package needs a leak check.
package fixture

import "golang.org/x/sync/errgroup"

func run() error {
	var g errgroup.Group
	g.Go(func() error { return nil }) // want `TS-C03: this package starts goroutines but its tests have no goleak check`
	return g.Wait()
}
