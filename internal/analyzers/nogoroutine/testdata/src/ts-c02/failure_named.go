// A named error result nothing assigns is still nil when it is returned, so
// that return is a success path.
package fixture

import "sync"

func returnsUnsetResult() (err error) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	return err
}
