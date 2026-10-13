// A goroutine per item inside a test file is the fan-out-then-wait idiom a
// concurrency test uses on purpose (ADR-0019), so it is silent.
package fixture

import (
	"sync"
	"testing"
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
