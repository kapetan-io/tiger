// Documented TS-C02 coverage gaps in function literals: the literal holding
// the Go runs after the Wait or alongside it, a t.Fatal skips the Wait, or a
// function around the literal returns only a non-nil error, which excuses the
// literal's own unwaited success path, a called literal returns an error
// before its Wait and the caller goes on, or a subtest reaches t.Parallel in a
// way the check does not see.
package fixture

import (
	"context"
	"subtesthelp"
	"sync"
	"testing"
)

// TestParallelSubtestOnTestGroup leaks the goroutine.
//
// known-miss: a parallel subtest runs after its parent returns, so its Go
// starts after the Wait that follows t.Run (ENG-802).
func TestParallelSubtestOnTestGroup(t *testing.T) {
	var wg sync.WaitGroup
	t.Run("x", func(t *testing.T) {
		t.Parallel()
		wg.Go(func() {})
	})
	wg.Wait()
}

// TestParallelSubtestOneLevelDown leaks the goroutine.
//
// known-miss: the same parallel subtest, with the group and the Wait in the
// outer subtest (ENG-802).
func TestParallelSubtestOneLevelDown(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		t.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Go(func() {})
		})
		wg.Wait()
	})
}

// TestSavedLiteralCalledAfterWait leaks the goroutine.
//
// known-miss: the literal is written before the Wait but called after it
// (ENG-809).
func TestSavedLiteralCalledAfterWait(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		run := func() { wg.Go(func() {}) }
		wg.Wait()
		run()
	})
}

// TestGoInCleanup leaks the goroutine.
//
// known-miss: t.Cleanup runs its literal after the test body, so the Go starts
// after the Wait (ENG-809).
func TestGoInCleanup(t *testing.T) {
	var wg sync.WaitGroup
	t.Cleanup(func() { wg.Go(func() {}) })
	wg.Wait()
}

// TestDeferredGoInSubtest leaks the goroutine.
//
// known-miss: the deferred literal runs when the subtest returns, so the Go
// starts after the Wait (ENG-809).
func TestDeferredGoInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		defer func() { wg.Go(func() {}) }()
		wg.Wait()
	})
}

// TestGoInOtherGroupInSubtest leaks the goroutine.
//
// known-miss: the Go runs in a goroutine of another group, so the Wait can run
// before it (ENG-809).
func TestGoInOtherGroupInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg, other sync.WaitGroup
		other.Go(func() { wg.Go(func() {}) })
		wg.Wait()
		other.Wait()
	})
}

// TestFatalBeforeWaitInSubtest leaks the goroutine.
//
// known-miss: t.Fatal ends the subtest's goroutine, so the Wait after it never
// runs (ENG-810).
func TestFatalBeforeWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {})
		if testing.Short() {
			t.Fatal("short")
		}
		wg.Wait()
	})
}

// namedErrorShadowed leaks the goroutine when the literal returns its own unset
// error before the Wait.
//
// known-miss: the function around the literal returns only its set error, so
// its failure path excuses the start inside the literal (ENG-805).
func namedErrorShadowed() (err error) {
	err = errStop
	run := func() (err error) {
		var wg sync.WaitGroup
		wg.Go(func() {})
		if testing.Short() {
			return err
		}
		wg.Wait()
		return nil
	}
	_ = run()
	return err
}

// literalNoWaitInFailingFunc leaks the goroutine: the literal reaches no Wait.
//
// known-miss: the function around the literal returns only a non-nil error, so
// its failure path excuses the start inside the literal (ENG-805).
func literalNoWaitInFailingFunc() error {
	run := func() {
		var wg sync.WaitGroup
		wg.Go(func() {})
	}
	run()
	return errStop
}

// TestCalledLiteralErrorSkipsWait leaks the goroutine: the literal returns an
// error before its Wait, and the test logs it and returns.
//
// known-miss: the literal's error return is not a success path, so the call
// counts as a Wait whatever the caller does with the error (ENG-966).
func TestCalledLiteralErrorSkipsWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	if err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}(); err != nil {
		t.Log(err)
	}
}

// TestParallelThroughCalledLiteralParam leaks the goroutine: the subtest is
// parallel, so its Wait runs after the test function returns.
//
// known-miss: t.Parallel runs on a literal's own parameter that the subtest's
// t is handed to, and the check follows only declared functions.
func TestParallelThroughCalledLiteralParam(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		func(inner *testing.T) { inner.Parallel() }(t)
		wg.Wait()
	})
}

// TestParallelThroughMethodValue leaks the goroutine the same way.
//
// known-miss: t.Parallel runs through a method value, which names no call
// on t.
func TestParallelThroughMethodValue(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		parallel := t.Parallel
		parallel()
		wg.Wait()
	})
}

// TestParallelThroughInterface leaks the goroutine the same way.
//
// known-miss: t.Parallel runs through an interface method, which resolves to
// no testing.T method.
func TestParallelThroughInterface(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		var marker interface{ Parallel() } = t
		marker.Parallel()
		wg.Wait()
	})
}

type subtestHolderField struct {
	t *testing.T
}

// TestParallelThroughField leaks the goroutine the same way.
//
// known-miss: t.Parallel runs on a struct field holding the subtest's t, and
// the check does not follow what a field holds.
func TestParallelThroughField(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		holder := subtestHolderField{t: t}
		holder.t.Parallel()
		wg.Wait()
	})
}

func markParallelTB(tb testing.TB) { tb.(*testing.T).Parallel() }

// TestParallelThroughTB leaks the goroutine the same way.
//
// known-miss: the helper takes testing.TB and asserts it back to *testing.T,
// so the call is not on the parameter it was handed.
func TestParallelThroughTB(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		markParallelTB(t)
		wg.Wait()
	})
}

// TestSkipBeforeWaitInSubtest leaks the goroutine when the subtest skips.
//
// known-miss: t.Skip ends the subtest, so the Wait after it never runs, but
// the check treats t.Skip as returning (ENG-810).
func TestSkipBeforeWaitInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		if testing.Short() {
			t.Skip("short")
		}
		wg.Wait()
	})
}

// TestParallelThroughOtherPackage leaks the goroutine: the subtest is
// parallel, so its Wait runs after the test function returns.
//
// known-miss: t.Parallel runs in a helper in another package, and the check
// follows only functions declared in the package it checks.
func TestParallelThroughOtherPackage(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		subtesthelp.Parallel(t)
		wg.Wait()
	})
}
