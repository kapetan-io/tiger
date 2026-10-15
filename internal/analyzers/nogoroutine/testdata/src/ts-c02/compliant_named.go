// A named error result that is assigned an error before the return is the
// failure path, which needs no Wait.
package fixture

import (
	"errors"
	"sync"
)

func returnsAssignedResult(fail bool) (err error) {
	var wg sync.WaitGroup
	wg.Go(work)
	if fail {
		err = errors.New("failed")
		return err
	}
	wg.Wait()
	return err
}
