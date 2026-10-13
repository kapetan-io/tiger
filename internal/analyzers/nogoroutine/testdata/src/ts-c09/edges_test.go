// More test-file loop shapes (ADR-0019): all silent for TS-C09.
package fixture

import (
	"sync"
	"testing"
)

func TestSubtestFanOut(t *testing.T) {
	var wg sync.WaitGroup
	for _, name := range []string{"a", "b"} {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func() { defer wg.Done() }()
			}
		})
	}
	wg.Wait()
}

func TestGoInsideRunInsideLoop(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run("x", func(t *testing.T) {
			go notifyOne("x")
		})
	}
}

func helperInTestFile() {
	for i := 0; i < 3; i++ {
		go notifyOne("x")
	}
}

func rangeFuncInTestFile(items []string) {
	for _, item := range items {
		func() {
			go notifyOne(item)
		}()
	}
}
