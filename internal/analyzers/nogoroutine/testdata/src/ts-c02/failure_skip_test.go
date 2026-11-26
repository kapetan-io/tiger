// Only t.Fatal, t.Fatalf and t.FailNow make a failure path. A path ending in
// t.Skip, t.Skipf, t.SkipNow or t.Error, or in a Fatal method that is not
// testing's, is a success path and must wait.
package fixture

import (
	"sync"
	"testing"
)

func TestWaitThenSkip(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		wg.Wait()
		return
	}
	t.Skip("not short")
}

func TestWaitThenSkipf(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		wg.Wait()
		return
	}
	t.Skipf("mode %v", testing.Verbose())
}

func TestWaitThenSkipNow(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		wg.Wait()
		return
	}
	t.SkipNow()
}

func TestWaitThenError(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		wg.Wait()
		return
	}
	t.Error("not short")
}

type fakeT struct{}

func (fakeT) Fatal(args ...any) {}

func TestWaitThenOtherFatal(t *testing.T) {
	var fake fakeT
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		wg.Wait()
		return
	}
	fake.Fatal("not short")
}
