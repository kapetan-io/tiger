// Loop shapes around a WaitGroup.Go call: each loop form, a call nested in a
// function literal, and a group nothing waits on.
package fixture

import "sync"

func notifyRangeInt(count int) {
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() { notifyOne("tick") }) // want `TS-C09: this loop starts a goroutine per item`
	}
	wg.Wait()
}

func notifyRangeFunc(entries func(yield func(string) bool)) {
	var wg sync.WaitGroup
	for entry := range entries {
		wg.Go(func() { notifyOne(entry) }) // want `TS-C09: this loop starts a goroutine per item`
	}
	wg.Wait()
}

// A function literal run per iteration is still the loop's work.
func notifyNested(entries []string) {
	var wg sync.WaitGroup
	for _, entry := range entries {
		func() {
			wg.Go(func() { notifyOne(entry) }) // want `TS-C09: this loop starts a goroutine per item`
		}()
	}
	wg.Wait()
}

// A loop inside a function literal around the call counts too.
func notifyLoopInLiteral(entries []string) {
	var wg sync.WaitGroup
	run := func() {
		for _, entry := range entries {
			wg.Go(func() { notifyOne(entry) }) // want `TS-C09: this loop starts a goroutine per item`
		}
	}
	run()
	wg.Wait()
}

// With no Wait reached both rules fire on the call: TS-C02 for the unowned
// group, TS-C09 for the loop.
func notifyUnwaited(entries []string) {
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Go(func() { notifyOne(entry) }) // want `TS-C02: this Go call starts a goroutine` `TS-C09: this loop starts a goroutine per item`
	}
}

// A method value that is never called starts nothing.
func methodValueNotCalled(count int) {
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		start := wg.Go
		_ = start
	}
	wg.Wait()
}
