// Documented TS-C02 coverage gaps in function literals: the literal holding
// the Go runs after the Wait, or a function around the literal returns only a
// non-nil error, which excuses the literal's own unwaited success path.
package fixture

import (
	"sync"
	"testing"
)

// TestParallelSubtestOnTestGroup leaks the goroutine.
//
// known-miss: a parallel subtest runs after its parent returns, so its Go
// starts after the Wait that follows t.Run.
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
// outer subtest.
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
// known-miss: the literal is written before the Wait but called after it.
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
// after the Wait.
func TestGoInCleanup(t *testing.T) {
	var wg sync.WaitGroup
	t.Cleanup(func() { wg.Go(func() {}) })
	wg.Wait()
}

// namedErrorShadowed leaks the goroutine when the literal returns its own unset
// error before the Wait.
//
// known-miss: the function around the literal returns only its set error, so
// its failure path excuses the start inside the literal.
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
// its failure path excuses the start inside the literal.
func literalNoWaitInFailingFunc() error {
	run := func() {
		var wg sync.WaitGroup
		wg.Go(func() {})
	}
	run()
	return errStop
}
