// Paths that end in t.Fatal, t.Fatalf or t.FailNow fail the test, like a
// returned non-nil error fails a function, so they need no Wait. Every success
// path still waits.
package fixture

import (
	"sync"
	"testing"
)

func TestWaitThenFatal(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	t.Fatal("not short")
}

func TestFatalfInDefault(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	switch {
	case testing.Short():
		wg.Wait()
	default:
		t.Fatalf("mode %v", testing.Verbose())
	}
}

func TestDeferredWaitBeforeFatal(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	defer wg.Wait()
	if testing.Short() {
		t.Fatal("short")
	}
}

func BenchmarkWaitThenFailNow(b *testing.B) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	b.FailNow()
}

func waitThenFatalTB(tb testing.TB) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	tb.Fatal("not short")
}

type fatalSuite struct {
	*testing.T
}

func (s fatalSuite) waitThenFatal() {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	s.Fatal("not short")
}

func TestFatalHelpers(t *testing.T) {
	waitThenFatalTB(t)
	fatalSuite{T: t}.waitThenFatal()
}
