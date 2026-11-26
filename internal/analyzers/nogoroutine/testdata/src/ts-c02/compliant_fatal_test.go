// Paths that end in t.Fatal, t.Fatalf or t.FailNow fail the test, like a
// returned non-nil error fails a function, so they need no Wait. The calls
// count on a *testing.T, *testing.B, *testing.F, testing.TB or a struct
// embedding one, in a test, a helper or a subtest. Every success path waits,
// and a deferred Wait runs when the test fails. A t.Fatal before the start
// ends nothing, since nothing has started, and a TryGo reads t.FailNow as a
// call that returns, so its path goes on to the Wait. A literal the test calls,
// where it is written or through a local variable, runs on the test's
// goroutine, so a t.Fatal in it ends the test as one written in place would.
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

func TestCalledLiteralFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	func() {
		if testing.Short() {
			t.Fatal("x")
		} else {
			wg.Wait()
		}
	}()
}

// TestSavedLiteralFatalOrWait replaces the saved literal only after the call,
// so the call runs the literal that may t.Fatal.
func TestSavedLiteralFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() {
		if testing.Short() {
			t.Fatal("x")
		} else {
			wg.Wait()
		}
	}
	wg.Go(work)
	wait()
	wait = func() {}
	_ = wait
}

func TestDeferredLiteralCallsFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	defer func() {
		func() {
			if testing.Short() {
				t.Fatal("x")
			} else {
				wg.Wait()
			}
		}()
	}()
}

func TestNestedCalledLiteralFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	func() {
		func() {
			if testing.Short() {
				t.Fatal("x")
			} else {
				wg.Wait()
			}
		}()
	}()
}

func TestSubtestStartThenCalledLiteralFatalOrWait(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(work)
		func() {
			if testing.Short() {
				t.Fatal("x")
			} else {
				wg.Wait()
			}
		}()
	})
}

func BenchmarkSubStartThenCalledLiteralFatalOrWait(b *testing.B) {
	b.Run("x", func(b *testing.B) {
		var wg sync.WaitGroup
		wg.Go(work)
		func() {
			if testing.Short() {
				b.Fatal("x")
			} else {
				wg.Wait()
			}
		}()
	})
}

func TestCalledLiteralFailNowOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	func() {
		if testing.Short() {
			t.FailNow()
		} else {
			wg.Wait()
		}
	}()
}

func TestCalledLiteralFatalfOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	func() {
		if testing.Short() {
			t.Fatalf("mode %v", testing.Verbose())
		} else {
			wg.Wait()
		}
	}()
}

func calledLiteralFatalOrWaitTB(tb testing.TB) {
	var wg sync.WaitGroup
	wg.Go(work)
	func() {
		if testing.Short() {
			tb.Fatal("x")
		} else {
			wg.Wait()
		}
	}()
}

func BenchmarkCalledLiteralFatalOrWait(b *testing.B) {
	var wg sync.WaitGroup
	wg.Go(work)
	func() {
		if testing.Short() {
			b.Fatal("x")
		} else {
			wg.Wait()
		}
	}()
}

func TestCalledLiteralTakesTFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work)
	func(t *testing.T) {
		if testing.Short() {
			t.Fatal("x")
		} else {
			wg.Wait()
		}
	}(t)
}
