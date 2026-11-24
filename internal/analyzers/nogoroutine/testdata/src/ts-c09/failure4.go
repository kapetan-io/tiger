// A WaitGroup.Go call per loop iteration starts a goroutine per item just
// as a go statement does, even when the group's Wait is reached.
package fixture

import "sync"

func notifyGroup(entries []string) {
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Go(func() { notifyOne(entry) }) // want `TS-C09: this loop starts a goroutine per item`
	}
	wg.Wait()
}

func notifyGroupCounted(count int) {
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Go(func() { notifyOne("tick") }) // want `TS-C09: this loop starts a goroutine per item`
	}
	wg.Wait()
}

// A Go call outside a loop stays accepted when its Wait is reached.
func notifyOnce(entry string) {
	var wg sync.WaitGroup
	wg.Go(func() { notifyOne(entry) })
	wg.Wait()
}

// Parentheses around the method do not hide the call.
func notifyGroupParen(entries []string) {
	var wg sync.WaitGroup
	for _, entry := range entries {
		(wg.Go)(func() { notifyOne(entry) }) // want `TS-C09: this loop starts a goroutine per item`
	}
	wg.Wait()
}
