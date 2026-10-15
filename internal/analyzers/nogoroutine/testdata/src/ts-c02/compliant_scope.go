// Compliant scoping cases: a group held in a field of a local is waited in the
// same function, and a goroutine started in a package-level function literal
// is waited there too.
package fixture

import "sync"

type scratch struct {
	wg sync.WaitGroup
}

func fieldOfLocal(f func()) {
	s := &scratch{}
	s.wg.Go(f)
	s.wg.Wait()
}

var packageLevel = func(f func()) {
	var wg sync.WaitGroup
	wg.Go(f)
	wg.Wait()
}
