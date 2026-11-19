// Documented TS-C02 coverage gaps in function literals: the literal holding
// the Go runs after the Wait or alongside it, a t.Fatal skips the Wait, or a
// function around the literal returns only a non-nil error, which excuses the
// literal's own unwaited success path.
package fixture

import (
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
