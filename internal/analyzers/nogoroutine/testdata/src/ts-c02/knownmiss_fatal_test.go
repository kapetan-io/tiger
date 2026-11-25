// Documented TS-C02 coverage gaps around calls that end the test's goroutine:
// a t.Fatal or t.Skip before the Wait skips it, and a helper that fails the
// test is a call that returns.
package fixture

import (
	"sync"
	"testing"
)

// TestFatalBeforeWait leaks the goroutine when the test fails.
//
// known-miss: a path ending in t.Fatal is a failure path and needs no Wait,
// though the goroutine outlives the failed test.
func TestFatalBeforeWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		t.Fatal("short")
	}
	wg.Wait()
}

// TestSkipBeforeWait leaks the goroutine when the test skips.
//
// known-miss: t.Skip ends the test, so the Wait after it never runs, but the
// check treats t.Skip as returning.
func TestSkipBeforeWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		t.Skip("short")
	}
	wg.Wait()
}

func requireLong(t *testing.T) {
	if testing.Short() {
		t.Fatal("short")
	}
}

// TestHelperFatalBeforeWait leaks the goroutine when the helper fails the
// test.
//
// known-miss: the helper fails the test on only some of its paths, so its call
// is a call that returns and the Wait after it counts.
func TestHelperFatalBeforeWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	requireLong(t)
	wg.Wait()
}
