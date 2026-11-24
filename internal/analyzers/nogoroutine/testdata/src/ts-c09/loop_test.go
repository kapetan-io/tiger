// A goroutine per item inside a test file is the fan-out-then-wait idiom a
// concurrency test uses on purpose (ADR-0019), so TS-C09 is silent here.
package fixture

import (
	"sync"
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestFanOut(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done() }()
	}
	for range 3 {
		go notifyOne("x")
	}
	wg.Wait()
}

// A spawn outside a loop is still an unowned goroutine, even in a test.
func TestUnowned(t *testing.T) {
	go notifyOne("x") // want `TS-C02: this go statement starts a goroutine nobody owns`
}

// A WaitGroup.Go call per item in a test file is silent too (ADR-0019).
func TestGroupFanOut(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {})
	}
	wg.Wait()
}

// An errgroup.Group start per item in a test file is silent too (ADR-0019).
func TestErrgroupFanOut(t *testing.T) {
	var group errgroup.Group
	for i := 0; i < 8; i++ {
		group.Go(func() error { return nil })
	}
	_ = group.Wait()
}

// A WaitGroup.Go call in a test loop with no Wait stays silent for TS-C09 and
// still fires TS-C02.
func TestGroupFanOutUnwaited(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine`
	}
}
