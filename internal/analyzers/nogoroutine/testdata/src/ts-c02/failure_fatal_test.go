// A t.Fatal on one path does not excuse the other paths: a start that no path
// waits on is still reported. A helper that fails the test is a call that
// returns, so the path goes on past it, and a subtest that fails ends only the
// subtest, so t.Run returns and its parent goes on without waiting.
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

func mustFailNow(t *testing.T) { t.Fatal("always") }

func TestHelperFailsNoWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	mustFailNow(t)
}

func TestSubtestFailsInsteadOfWaiting(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("wait", func(t *testing.T) {
		if testing.Short() {
			t.Fatal("short")
		} else {
			wg.Wait()
		}
	})
}
