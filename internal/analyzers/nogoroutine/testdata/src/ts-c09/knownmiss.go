// Documented gap: a Go method saved in a variable and called in a loop starts
// a goroutine per item, but TS-C09 sees only a call on the selector itself, so
// it stays silent.
package fixture

import "sync"

func notifyMethodValue(entries []string) {
	var wg sync.WaitGroup
	start := wg.Go
	for _, entry := range entries {
		start(func() { notifyOne(entry) })
	}
	wg.Wait()
}

func notifyMethodValueInLoop(entries []string) {
	var wg sync.WaitGroup
	for _, entry := range entries {
		start := wg.Go
		start(func() { notifyOne(entry) })
	}
	wg.Wait()
}
