// A t.Fatal on one path does not excuse the other paths: a start that no path
// waits on is still reported.
package fixture

import (
	"sync"
	"testing"
)

func TestFatalNoWaitAnywhere(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		t.Fatal("short")
	}
}
