// Paths that end in t.Fatal, t.Fatalf or t.FailNow fail the test, like a
// returned non-nil error fails a function, so they need no Wait. The calls
// count on a *testing.T, *testing.B, *testing.F, testing.TB or a struct
// embedding one, in a test, a helper or a subtest. Every success path waits,
// and a deferred Wait runs when the test fails. A t.Fatal before the start
// ends nothing, since nothing has started, and a TryGo reads t.FailNow as a
// call that returns, so its path goes on to the Wait.
package fixture

import (
	"sync"
	"testing"

	"golang.org/x/sync/errgroup"
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

func FuzzWaitThenFatal(f *testing.F) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	f.Fatal("not short")
}

func waitThenFailNowTB(tb testing.TB) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	tb.FailNow()
}

func waitThenFatalfTB(tb testing.TB) {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	tb.Fatalf("mode %v", testing.Verbose())
}

type fatalTBSuite struct {
	testing.TB
}

func (s fatalTBSuite) waitThenFatal() {
	var wg sync.WaitGroup
	wg.Go(work)
	if testing.Short() {
		wg.Wait()
		return
	}
	s.Fatal("not short")
}

func TestFatalInSubtest(t *testing.T) {
	waitThenFailNowTB(t)
	waitThenFatalfTB(t)
	fatalTBSuite{TB: t}.waitThenFatal()
	t.Run("fatal", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(work)
		if testing.Short() {
			wg.Wait()
			return
		}
		t.Fatal("not short")
	})
}

func TestSubtestWaitsThenFatal(t *testing.T) {
	t.Run("fatal", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(work)
		wg.Wait()
		t.Fatal("after wait")
	})
}

func TestFatalBeforeStartInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	t.Run("x", func(t *testing.T) {
		t.Fatal("always")
		wg.Go(work)
		wg.Wait()
	})
}

func TestTryGoFailNowBeforeWait(t *testing.T) {
	var g errgroup.Group
	g.TryGo(func() error { return nil })
	t.FailNow()
	_ = g.Wait()
}

func TestFatalInsideCleanupAfterStart(t *testing.T) {
	var wg sync.WaitGroup
	t.Cleanup(func() {
		wg.Go(work)
		t.Fatal("cleanup failed")
	})
}

func TestFatalInsideDeferAfterStart(t *testing.T) {
	var wg sync.WaitGroup
	defer func() {
		wg.Go(work)
		t.Fatal("deferred failed")
	}()
}

func TestFatalInsideSubtestAfterStart(t *testing.T) {
	var wg sync.WaitGroup
	t.Run("x", func(t *testing.T) {
		wg.Go(work)
		t.Fatal("subtest failed")
	})
}
