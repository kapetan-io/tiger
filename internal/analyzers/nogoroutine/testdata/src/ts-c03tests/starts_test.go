package fixture

import (
	"sync"
	"testing"
)

func TestAddConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() { _ = Add(1, 2) })
}
