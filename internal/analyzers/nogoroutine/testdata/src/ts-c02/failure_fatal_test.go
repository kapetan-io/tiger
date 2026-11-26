// A t.Fatal on one path does not excuse the other paths: a start that no path
// waits on is still reported. A helper that fails the test is a call that
// returns, so the path goes on past it, and a subtest that fails ends only the
// subtest, so t.Run returns and its parent goes on without waiting. A start in
// a deferred or t.Cleanup literal runs after the t.Fatal, so that t.Fatal
// excuses nothing, even when the t.Fatal sits in a literal the test calls. A
// t.Fatal in a literal that a t.Run or b.Run body calls fails only the
// subtest, so the parent goes on without waiting.
package fixture

import (
	"sync"
	"testing"
)

func TestFatalNoWaitAnywhere(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		t.Fatal("short")
	}
}

func mustFailNow(t *testing.T) { t.Fatal("always") }

func TestHelperFailsNoWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	mustFailNow(t)
}

func TestSubtestFailsInsteadOfWaiting(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("wait", func(t *testing.T) {
		if testing.Short() {
			t.Fatal("short")
		} else {
			wg.Wait()
		}
	})
}

func TestDeferredStartThenFatal(t *testing.T) {
	var wg sync.WaitGroup
	defer func() {
		wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	}()
	t.Fatal("before the deferred start runs")
}

func TestCleanupStartThenFatal(t *testing.T) {
	var wg sync.WaitGroup
	t.Cleanup(func() {
		wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	})
	t.Fatal("before the cleanup start runs")
}

func TestSubtestCallsLiteralFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("wait", func(t *testing.T) {
		func() {
			if testing.Short() {
				t.Fatal("short")
			} else {
				wg.Wait()
			}
		}()
	})
}

func TestDeferredStartThenCalledLiteralFatal(t *testing.T) {
	var wg sync.WaitGroup
	defer func() {
		wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	}()
	func() {
		if testing.Short() {
			t.Fatal("before the deferred start runs")
		} else {
			wg.Wait()
		}
	}()
}

func TestSubtestCallsSavedLiteralFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("wait", func(t *testing.T) {
		wait := func() {
			if testing.Short() {
				t.Fatal("short")
			} else {
				wg.Wait()
			}
		}
		wait()
	})
}

func TestSubtestCallsNestedLiteralFatalOrWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("wait", func(t *testing.T) {
		func() {
			func() {
				if testing.Short() {
					t.Fatal("short")
				} else {
					wg.Wait()
				}
			}()
		}()
	})
}

func BenchmarkSubCallsLiteralFatalOrWait(b *testing.B) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	b.Run("wait", func(b *testing.B) {
		func() {
			if testing.Short() {
				b.Fatal("short")
			} else {
				wg.Wait()
			}
		}()
	})
}

func TestCleanupStartThenCalledLiteralFatal(t *testing.T) {
	var wg sync.WaitGroup
	t.Cleanup(func() {
		wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	})
	func() {
		if testing.Short() {
			t.Fatal("before the cleanup start runs")
		} else {
			wg.Wait()
		}
	}()
}

func TestDeferredStartThenNestedLiteralFatal(t *testing.T) {
	var wg sync.WaitGroup
	defer func() {
		wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	}()
	func() {
		func() {
			if testing.Short() {
				t.Fatal("before the deferred start runs")
			} else {
				wg.Wait()
			}
		}()
	}()
}

func TestCalledLiteralFatalNoWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	func() {
		if testing.Short() {
			t.Fatal("short")
		}
	}()
}

// BenchmarkUnnamedSubCallsParentFatalOrWait leaks on the b.Fatal path: b.Run
// returns to the parent even when its body calls the parent's b.Fatal.
func BenchmarkUnnamedSubCallsParentFatalOrWait(b *testing.B) {
	var wg sync.WaitGroup
	wg.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	b.Run("wait", func(*testing.B) {
		if testing.Short() {
			b.Fatal("short")
		} else {
			wg.Wait()
		}
	})
}
