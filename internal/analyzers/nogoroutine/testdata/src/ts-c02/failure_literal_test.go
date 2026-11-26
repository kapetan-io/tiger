// TS-C02 findings inside function literals: no Wait follows the Go on every
// success path of the literal or a function around it, the Wait runs inside
// the group's own goroutine, or the Wait sits in a literal that may not run
// before the call holding it returns (saved and never called, replaced on
// some path before the call, passed to another function, a parallel subtest
// that no t.Run the function holding the Go calls waits for, or t.Cleanup),
// or the literal returns an error before its Wait and the caller drops that
// error or goes on.
package fixture

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"golang.org/x/sync/errgroup"
)

func wrap(f func()) func() { return f }

func TestNoWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	})
}

func TestNestedSubtestNoWait(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		t.Run("y", func(t *testing.T) {
			wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		})
	})
}

func TestWaitBeforeGoInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Wait()
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	})
}

func TestWaitBeforeSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Wait()
	t.Run("x", func(t *testing.T) {
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	})
}

func TestWaitOnOneBranchInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		if testing.Short() {
			return
		}
		wg.Wait()
	})
}

func TestSavedWaitOnOneBranch(t *testing.T) {
	run := func() {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		if testing.Short() {
			wg.Wait()
		}
	}
	run()
}

func TestReturnNilBeforeWaitInLiteral(t *testing.T) {
	run := func() error {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		if testing.Short() {
			return nil
		}
		wg.Wait()
		return nil
	}
	_ = run()
}

// TestWaitInUncalledLiteral starts in the subtest and waits only in a
// literal that is saved and never called.
func TestWaitInUncalledLiteral(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wait := func() { wg.Wait() }
		_ = wait
	})
}

func TestSavedNeverCalledNoWait(t *testing.T) {
	run := func() {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	}
	_ = run
}

func TestPassedAlongNoWait(t *testing.T) {
	ignore(func() {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	})
}

// TestWaitOnlyInCleanup waits only in t.Cleanup, which runs after the test
// body returns, so the goroutine outlives the body.
func TestWaitOnlyInCleanup(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Cleanup(func() {
		wg.Wait()
	})
}

// TestWaitInsideOwnGoroutine waits from inside a goroutine of the same group,
// so the Wait waits on itself and never returns.
func TestWaitInsideOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Wait()
	})
}

// TestWaitNestedInOwnGoroutine waits from a subtest that runs inside a
// goroutine of the same group.
func TestWaitNestedInOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		t.Run("x", func(t *testing.T) {
			wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
			wg.Wait()
		})
	})
}

// TestWaitInParenthesizedOwnGoroutine waits from a goroutine of the same
// group whose literal sits in parentheses.
func TestWaitInParenthesizedOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go((func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Wait()
	}))
}

// TestWaitInConvertedOwnGoroutine waits from a goroutine of the same group
// whose literal is converted to func() first.
func TestWaitInConvertedOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go((func())(func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Wait()
	}))
}

// TestWaitInAddressedOwnGoroutine waits from a goroutine of the same group
// started through the group's address.
func TestWaitInAddressedOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	(&wg).Go(func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Wait()
	})
}

// failingGoroutine starts in a goroutine literal whose only exit returns a
// non-nil error, so the literal has no success path to Wait on.
func failingGoroutine() error {
	var g errgroup.Group
	g.Go(func() error {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		return errStop
	})
	return g.Wait()
}

// foreverGoroutine starts in a goroutine literal that never returns, so the
// literal has no success path to Wait on.
func foreverGoroutine(ch <-chan int) {
	var outer sync.WaitGroup
	outer.Go(func() {
		var inner sync.WaitGroup
		inner.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		for {
			<-ch
		}
	})
	outer.Wait()
}

// TestWaitInAliasedOwnGoroutine starts its goroutine through a pointer that
// may hold the group, so a Wait inside it may wait on itself.
func TestWaitInAliasedOwnGoroutine(t *testing.T) {
	var wg, other sync.WaitGroup
	p := &wg
	if testing.Short() {
		p = &other
	}
	p.Go(func() {
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Wait()
	})
	p.Wait()
}

// TestWaitInMethodExpressionOwnGoroutine starts its goroutine through a
// method expression on the group.
func TestWaitInMethodExpressionOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	(*sync.WaitGroup).Go(&wg, func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Wait()
	})
}

// TestSelfWaitThroughMethodValue waits on the group from inside its own
// goroutine.
//
// known-miss: the literal reaches wg.Go through a method value, so the check
// does not see it runs as the group's goroutine, and the inner Go passes
// (ENG-804).
func TestSelfWaitThroughMethodValue(t *testing.T) {
	var wg sync.WaitGroup
	start := wg.Go // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	start(func() {
		wg.Go(func() {})
		wg.Wait()
	})
}

// TestTryGoWaitsOnItself waits on the group from inside the goroutine TryGo
// starts on it.
func TestTryGoWaitsOnItself(t *testing.T) {
	var g errgroup.Group
	g.TryGo(func() error { // want `TS-C02: this TryGo call starts a goroutine, but no Wait on the group is reached`
		g.Go(func() error { return nil }) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		return g.Wait()
	})
}

func TestBareGoAroundWaitedGroup(t *testing.T) {
	go func() { // want `TS-C02: this go statement starts a goroutine nobody owns`
		var wg sync.WaitGroup
		wg.Go(func() {})
		wg.Wait()
	}()
}

// TestSelfWaitThroughVariable waits on the group from inside its own
// goroutine.
//
// known-miss: the literal reaches wg.Go through a variable, so the check does
// not see it runs as the group's goroutine, and the inner Go passes (ENG-804).
func TestSelfWaitThroughVariable(t *testing.T) {
	var wg sync.WaitGroup
	body := func() {
		wg.Go(func() {})
		wg.Wait()
	}
	wg.Go(body) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// TestSelfWaitThroughWrap waits on the group from inside its own goroutine.
//
// known-miss: the literal reaches wg.Go through a call, so the check does not
// see it runs as the group's goroutine, and the inner Go passes (ENG-804).
func TestSelfWaitThroughWrap(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(wrap(func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wg.Go(func() {})
		wg.Wait()
	}))
}

// TestNamedErrorUnsetBeforeWaitInLiteral returns its named error result
// before setting it, so the result is nil and the path is a success.
func TestNamedErrorUnsetBeforeWaitInLiteral(t *testing.T) {
	run := func() (err error) {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		if testing.Short() {
			return err
		}
		wg.Wait()
		return nil
	}
	_ = run()
}

// TestNamedErrorNilBeforeWaitInLiteral sets its named error result to nil
// before returning it, so the path is a success.
func TestNamedErrorNilBeforeWaitInLiteral(t *testing.T) {
	run := func() (err error) {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		if testing.Short() {
			err = nil
			return err
		}
		wg.Wait()
		return nil
	}
	_ = run()
}

// namedErrorNilBeforeWait is the same path in a declaration.
func namedErrorNilBeforeWait() (err error) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		err = nil
		return err
	}
	wg.Wait()
	return nil
}

type subtestNoWait struct {
	wg sync.WaitGroup
}

func (h *subtestNoWait) runAll(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		h.wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	})
}

func TestTryGoNoWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var g errgroup.Group
		g.TryGo(func() error { return nil }) // want `TS-C02: this TryGo call starts a goroutine, but no Wait on the group is reached`
	})
}

// TestExpiredWaitInSubtest waits only after ctx has expired.
func TestExpiredWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		ctx := context.Background()
		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		select {
		case <-done:
			return
		case <-ctx.Done():
			wg.Wait()
		}
	})
}

// TestGoStatementInSubtest starts on the group from a bare goroutine, which
// the go statement finding reports.
func TestGoStatementInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		go func() { // want `TS-C02: this go statement starts a goroutine nobody owns`
			wg.Go(func() {})
		}()
		wg.Wait()
	})
}

var runPackageLiteralNoWait = func() {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// TestWaitOnlyInParallelSubtest waits in a subtest that calls t.Parallel, so
// the Wait runs after the test function returns.
func TestWaitOnlyInParallelSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		t.Parallel()
		wg.Wait()
	})
}

// TestWaitInPassedLiteral passes the waiting literal to a function that may
// not call it.
func TestWaitInPassedLiteral(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	ignore(func() { wg.Wait() })
}

// TestSavedWaitCalledOnOneBranch calls the saved literal on one branch only.
func TestSavedWaitCalledOnOneBranch(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if testing.Short() {
		wait()
	}
}

// TestSavedWaitCalledBeforeGo calls the saved literal before the Go.
func TestSavedWaitCalledBeforeGo(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wait()
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// TestReassignedSavedWait may call a literal that does not wait, because the
// variable is reassigned.
func TestReassignedSavedWait(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	if testing.Short() {
		wait = func() {}
	}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestSavedWaitPassedAlong passes the saved waiting literal to a function
// that may not call it.
func TestSavedWaitPassedAlong(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	ignore(wait)
}

// TestSubtestWaitInsideOwnGoroutine waits from a subtest inside a goroutine
// of the same group.
func TestSubtestWaitInsideOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		t.Run("x", func(t *testing.T) {
			wg.Wait()
		})
	})
}

// TestSavedWaitInsideOwnGoroutine calls a saved waiting literal from inside a
// goroutine of the same group.
func TestSavedWaitInsideOwnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() { // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		wait()
	})
}

// TestSubtestWaitsOnOneBranch waits in a later subtest that may return first.
func TestSubtestWaitsOnOneBranch(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		if testing.Short() {
			return
		}
		wg.Wait()
	})
}

// TestCalledLiteralWaitsOnOneBranch calls a literal where it is written that
// waits on one branch only.
func TestCalledLiteralWaitsOnOneBranch(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	func() {
		if testing.Short() {
			wg.Wait()
		}
	}()
}

// reassignedParameterWait calls the caller's function before the parameter
// is replaced with a literal that waits.
func reassignedParameterWait(wait func()) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
	wait = func() { wg.Wait() }
	_ = wait
}

var packageWait = func() {}

// TestPackageLiteralCalled calls a package variable that may hold any
// function by the time it runs.
func TestPackageLiteralCalled(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	packageWait = func() { wg.Wait() }
	packageWait()
}

// TestSavedWaitNeverCalled saves a waiting literal after the Go and never
// calls it.
func TestSavedWaitNeverCalled(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait := func() { wg.Wait() }
	_ = wait
}

// TestSavedWaitInCleanup hands the saved waiting literal to t.Cleanup, which
// runs it after the test body returns.
func TestSavedWaitInCleanup(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Cleanup(wait)
}

// TestSavedParallelSubtestBody passes a saved subtest body that calls
// t.Parallel.
func TestSavedParallelSubtestBody(t *testing.T) {
	var wg sync.WaitGroup
	body := func(t *testing.T) {
		t.Parallel()
		wg.Wait()
	}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", body)
}

// TestParallelInCalledLiteralInSubtest calls t.Parallel from a literal called
// where it is written, so the subtest is still parallel.
func TestParallelInCalledLiteralInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		func() { t.Parallel() }()
		wg.Wait()
	})
}

func markParallel(t *testing.T) { t.Parallel() }

// TestParallelThroughHelperInSubtest calls t.Parallel through a helper, so
// the subtest is still parallel.
func TestParallelThroughHelperInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		markParallel(t)
		wg.Wait()
	})
}

// TestSavedWaitReplacedThroughPointer replaces the saved literal through a
// pointer before calling it.
func TestSavedWaitReplacedThroughPointer(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	slot := &wait
	*slot = func() {}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestSubtestWaitsOnOtherGroup waits on a different group in the subtest.
func TestSubtestWaitsOnOtherGroup(t *testing.T) {
	var wg, other sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		other.Wait()
	})
}

// TestSubtestWaitsInGoStatement waits in a goroutine the subtest starts, so
// the subtest returns before the Wait.
func TestSubtestWaitsInGoStatement(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		go wg.Wait() // want `TS-C02: this go statement starts a goroutine nobody owns`
	})
}

// TestStartLiteralReachedTwoHops calls the literal that waits and then starts
// through another literal, so the Go runs after the Wait.
func TestStartLiteralReachedTwoHops(t *testing.T) {
	var wg sync.WaitGroup
	later := func() {
		wg.Wait()
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	}
	run := func() { later() }
	run()
}

// TestStartLiteralReachedFromSubtest reaches the same literal from a subtest.
func TestStartLiteralReachedFromSubtest(t *testing.T) {
	var wg sync.WaitGroup
	later := func() {
		wg.Wait()
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	}
	t.Run("x", func(t *testing.T) { later() })
}

// TestSavedWaitReplacedByRange replaces the saved literal in a range clause
// before calling it.
func TestSavedWaitReplacedByRange(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	for _, wait = range []func(){func() {}} {
	}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestDeferredLiteralWaitsOnOneBranch defers a literal that waits on one
// branch only.
func TestDeferredLiteralWaitsOnOneBranch(t *testing.T) {
	var wg sync.WaitGroup
	defer func() {
		if testing.Short() {
			wg.Wait()
		}
	}()
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// TestSavedWaitReplacedThroughPointerBeforeRange replaces the saved literal
// through a pointer, with a range statement after it.
func TestSavedWaitReplacedThroughPointerBeforeRange(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	slot := &wait
	*slot = func() {}
	for range 1 {
	}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestSavedWaitReplacedThroughPointerPair takes the literal's address in a
// two-value assignment.
func TestSavedWaitReplacedThroughPointerPair(t *testing.T) {
	var wg sync.WaitGroup
	var other int
	wait := func() { wg.Wait() }
	slot, _ := &wait, &other
	*slot = func() {}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

func markParallelThroughHelper(t *testing.T) { markParallel(t) }

// TestParallelTwoHelpersDownInSubtest calls t.Parallel two helpers down.
func TestParallelTwoHelpersDownInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		markParallelThroughHelper(t)
		wg.Wait()
	})
}

type parallelSuite struct{}

func (parallelSuite) setup(t *testing.T) { t.Parallel() }

// TestParallelThroughMethodInSubtest calls t.Parallel through a method.
func TestParallelThroughMethodInSubtest(t *testing.T) {
	var suite parallelSuite
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		suite.setup(t)
		wg.Wait()
	})
}

// TestSavedGoroutineBodyCalledAfter waits on the group from inside its own
// goroutine, then calls the same literal.
//
// known-miss: the literal reaches wg.Go through a variable, so the check does
// not see it runs as the group's goroutine, and the call after counts as the
// Wait (ENG-804).
func TestSavedGoroutineBodyCalledAfter(t *testing.T) {
	var wg sync.WaitGroup
	body := func() { wg.Wait() }
	wg.Go(body)
	body()
}

// TestParallelThroughMethodExpressionInSubtest calls t.Parallel through a
// method expression, whose first argument is the receiver.
func TestParallelThroughMethodExpressionInSubtest(t *testing.T) {
	var suite parallelSuite
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		parallelSuite.setup(suite, t)
		wg.Wait()
	})
}

func markParallelGeneric[T any](t *testing.T) { t.Parallel() }

// TestParallelThroughGenericHelperInSubtest calls t.Parallel through an
// explicitly instantiated generic helper.
func TestParallelThroughGenericHelperInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		markParallelGeneric[int](t)
		wg.Wait()
	})
}

// TestParallelThroughAliasInSubtest calls t.Parallel on a copy of the
// subtest's t.
func TestParallelThroughAliasInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		alias := t
		alias.Parallel()
		wg.Wait()
	})
}

type embeddedSubtest struct {
	*testing.T
}

// TestParallelThroughEmbeddingInSubtest calls t.Parallel through a struct
// that embeds the subtest's t.
func TestParallelThroughEmbeddingInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		suite := embeddedSubtest{t}
		suite.Parallel()
		wg.Wait()
	})
}

// TestParallelSubtestOnTestFromSubtest starts the parallel subtest on the
// test's own t from inside a plain subtest, so it runs after the test
// function returns.
func TestParallelSubtestOnTestFromSubtest(t *testing.T) {
	var wg sync.WaitGroup
	outer := t
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		outer.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestParallelSubtestInParallelSubtest waits two levels down, but the outer
// subtest is parallel too, so both run after the test function returns.
func TestParallelSubtestInParallelSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		t.Parallel()
		t.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestSavedWaitReplacedBeforeNextPass is an expected false positive
// (ENG-989): the first pass always runs the literal that waits, and every path
// out of the loop passes that call, but a later pass reaches the call holding
// the replacement, and the check does not count a call that holds more than
// one literal.
func TestSavedWaitReplacedBeforeNextPass(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	for {
		wait()
		wait = func() {}
		if testing.Short() {
			break
		}
	}
}

// TestSavedWaitReplacedInCalledLiteral replaces the saved literal inside
// another literal. An assignment in another literal is not followed, so the
// call may run the literal that does not wait.
func TestSavedWaitReplacedInCalledLiteral(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	func() { wait = func() {} }()
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestParallelSubtestOnOuterSubtestFromParallelSubtest starts a parallel
// subtest on the outer subtest's t from inside a parallel subtest. "x" has
// already stopped waiting for its parallel subtests, so t.Run("x") returns
// before the Wait runs.
func TestParallelSubtestOnOuterSubtestFromParallelSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(outer *testing.T) {
		outer.Run("y", func(t *testing.T) {
			t.Parallel()
			outer.Run("z", func(t *testing.T) {
				t.Parallel()
				wg.Wait()
			})
		})
	})
}

// TestSavedWaitReplacedInGoroutine replaces the saved literal from a
// goroutine of another group, which may run before the call.
func TestSavedWaitReplacedInGoroutine(t *testing.T) {
	var wg, other sync.WaitGroup
	wait := func() { wg.Wait() }
	other.Go(func() { wait = func() {} })
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
	other.Wait()
}

// TestSavedWaitReplacedInSelectCase replaces the saved literal in one select
// case before the call.
func TestSavedWaitReplacedInSelectCase(t *testing.T) {
	var wg sync.WaitGroup
	ready := make(chan struct{})
	wait := func() { wg.Wait() }
	select {
	case <-ready:
		wait = func() {}
	default:
	}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestSavedWaitReplacedBeforeContinueOuter replaces the saved literal and
// continues a labeled outer loop, so the call after the loops may run the
// literal that does not wait.
func TestSavedWaitReplacedBeforeContinueOuter(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
outer:
	for range 2 {
		for {
			if testing.Short() {
				wait = func() {}
				continue outer
			}
			break
		}
	}
	wait()
}

// TestSavedWaitAssignmentSkippedByGoto jumps over the assignment of the
// waiting literal, so the call runs the one that does not wait.
func TestSavedWaitAssignmentSkippedByGoto(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() {}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	goto call
	wait = func() { wg.Wait() }
call:
	wait()
}

// TestDeferredSavedNoWaitReplacedAfterDefer defers the saved literal while it
// does not wait. The defer keeps that value, so replacing it later does not
// help.
func TestDeferredSavedNoWaitReplacedAfterDefer(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() {}
	defer wait()
	wait = func() { wg.Wait() }
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

// TestSavedWaitReplacedThenShadowed replaces the saved literal on one branch,
// then declares a shadowing variable that holds a waiting literal.
func TestSavedWaitReplacedThenShadowed(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	if testing.Short() {
		wait = func() {}
	}
	{
		wait := func() { wg.Wait() }
		_ = wait
	}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestSavedWaitReplacedThenCalledInLiteral replaces the saved literal before
// a literal called where it is written calls it.
func TestSavedWaitReplacedThenCalledInLiteral(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wait = func() {}
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	func() { wait() }()
}

// TestGoInSubtestWaitInItsParallelSubtest starts in a plain subtest and waits
// in a parallel subtest of it, which runs after that subtest's body returns.
func TestGoInSubtestWaitInItsParallelSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
		t.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestParallelSubtestOnReassignedSubtestParam replaces the subtest's t with
// the test's own before starting the parallel subtest on it.
func TestParallelSubtestOnReassignedSubtestParam(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(st *testing.T) {
		st = t
		st.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestParallelSubtestOnAliasReplacedThroughPointer replaces a copy of the
// subtest's t with the test's own through a pointer.
func TestParallelSubtestOnAliasReplacedThroughPointer(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(st *testing.T) {
		alias := st
		slot := &alias
		*slot = t
		alias.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestParallelSubtestOnStaleAlias keeps the first run's t in a variable
// declared outside the subtest body, so the second run starts its parallel
// subtest on a subtest that has already finished.
func TestParallelSubtestOnStaleAlias(t *testing.T) {
	var wg sync.WaitGroup
	var keep *testing.T
	body := func(t *testing.T) {
		if keep == nil {
			keep = t
		}
		keep.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	}
	t.Run("x", body)
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x2", body)
}

// TestParallelSubtestInStaleSavedLiteral saves a literal holding the first
// run's t in a variable declared outside the subtest body, so the second run
// calls it on a subtest that has already finished.
func TestParallelSubtestInStaleSavedLiteral(t *testing.T) {
	var wg sync.WaitGroup
	var saved func()
	body := func(t *testing.T) {
		if saved == nil {
			saved = func() {
				t.Run("y", func(t *testing.T) {
					t.Parallel()
					wg.Wait()
				})
			}
		}
		saved()
	}
	t.Run("x", body)
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x2", body)
}

// TestSavedWaitCalledFromCalledLiteral is an expected false positive
// (ENG-989): the call sits in a literal called where it is written, after the
// last assignment, so it runs the literal that waits, but the check follows a
// literal assigned more than once only within one function body.
func TestSavedWaitCalledFromCalledLiteral(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() {}
	wait = func() { wg.Wait() }
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	func() { wait() }()
}

// TestSavedWaitReplacedInDeferredLiteral is an expected false positive
// (ENG-989): the deferred literal replaces the saved one only after the call,
// but the check does not follow an assignment inside another literal.
func TestSavedWaitReplacedInDeferredLiteral(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	defer func() { wait = func() {} }()
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
}

// TestCalledLiteralErrorSkipsWait leaks the goroutine: the literal returns an
// error before its Wait, and the test logs it and returns.
func TestCalledLiteralErrorSkipsWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
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

// TestCalledLiteralErrorDropped leaks the goroutine: the literal's error is
// thrown away, so the test goes on after the path that skipped the Wait.
func TestCalledLiteralErrorDropped(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	_ = func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
}

// TestSavedLiteralErrorIgnored leaks the goroutine: the saved literal's error
// is never looked at.
func TestSavedLiteralErrorIgnored(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	run := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}
	run()
}

// TestVarLiteralErrorLogged binds the literal's error with var and only logs
// it.
func TestVarLiteralErrorLogged(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	var err = func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	if err != nil {
		t.Log(err)
	}
}

// TestLiteralErrorOverwritten replaces the literal's error before the check,
// so the test goes on after the path that skipped the Wait.
func TestLiteralErrorOverwritten(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	err = context.Background().Err()
	if err != nil {
		t.Fatal(err)
	}
}

// TestLiteralErrorFailsOnlyWithAnd fails only when a second condition also
// holds, so the test goes on after the error otherwise.
func TestLiteralErrorFailsOnlyWithAnd(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}(); err != nil && testing.Verbose() {
		t.Fatal(err)
	}
}

// TestLiteralErrorFailsSubtestOnly runs the literal in a subtest; t.Fatal ends
// only the subtest, and the test function goes on.
func TestLiteralErrorFailsSubtestOnly(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	t.Run("x", func(t *testing.T) {
		if err := func() error {
			if testing.Short() {
				return context.Canceled
			}
			wg.Wait()
			return nil
		}(); err != nil {
			t.Fatal(err)
		}
	})
}

// TestLiteralErrorLoggedInSwitch only logs the literal's error from a switch
// case.
func TestLiteralErrorLoggedInSwitch(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	switch {
	case err != nil:
		t.Log(err)
	}
}

// TestErrgroupLiteralErrorLogged returns before the errgroup's Wait and the
// test only logs the error.
func TestErrgroupLiteralErrorLogged(t *testing.T) {
	var g errgroup.Group
	g.Go(func() error { return nil }) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		return g.Wait()
	}(); err != nil {
		t.Log(err)
	}
}

// TestNestedLiteralErrorLogged passes an inner literal's error up through an
// outer literal, and the test only logs it.
func TestNestedLiteralErrorLogged(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if err := func() error {
		return func() error {
			if testing.Short() {
				return context.Canceled
			}
			wg.Wait()
			return nil
		}()
	}(); err != nil {
		t.Log(err)
	}
}

// literalErrorClearedOnBranch leaks the goroutine when testing.Short clears
// the literal's error before the check.
func literalErrorClearedOnBranch() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Verbose() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	if testing.Short() {
		err = nil
	}
	if err != nil {
		return err
	}
	return nil
}

// calledLiteralErrorSwallowed leaks the goroutine: the error branch returns
// nil, so the caller's caller sees success.
func calledLiteralErrorSwallowed() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}(); err != nil {
		return nil
	}
	return nil
}

func requireNoError(t testing.TB, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

// TestLiteralErrorRequired is an expected false positive (ENG-994): the
// helper fails the test on the literal's error, but the check reads it as a
// call that returns.
func TestLiteralErrorRequired(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	requireNoError(t, func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}())
}

// TestLiteralErrorSwitched is an expected false positive (ENG-994): the
// switch fails the test on a non-nil error, but only a comparison with nil
// is read.
func TestLiteralErrorSwitched(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	switch err {
	case nil:
	default:
		t.Fatal(err)
	}
}

// literalErrorPanics is an expected false positive (ENG-993): the error
// branch panics, but a path ending in panic is read as a success path.
func literalErrorPanics() {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}(); err != nil {
		panic(err)
	}
}

// literalErrorLoggedThenOtherReturn leaks the goroutine: the error branch only
// logs, and the later return carries another error.
func literalErrorLoggedThenOtherReturn(t *testing.T) error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	if err != nil {
		t.Log(err)
	}
	return errors.Join(errors.ErrUnsupported)
}

// namedLiteralErrorReplaced leaks the goroutine: the named result no longer
// holds the literal's error when it is returned.
func namedLiteralErrorReplaced(t *testing.T) (err error) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err = func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	if err != nil {
		t.Log(err)
	}
	err = errors.ErrUnsupported
	return
}

// TestDeferredLiteralErrorDropped leaks the goroutine: the deferred literal's
// error is thrown away, so the path that skipped the Wait goes unnoticed.
func TestDeferredLiteralErrorDropped(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	defer func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
}

func ignoreAll(func()) error {
	return nil
}

// literalErrorOnlyInClosure leaks the goroutine: the returned call
// mentions the error only inside a closure it never returns.
func literalErrorOnlyInClosure() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	if err != nil {
		return ignoreAll(func() { _ = err })
	}
	return nil
}

func maybe(bool) error {
	return nil
}

// literalErrorAsBool leaks the goroutine: the error branch
// returns a call that sees only whether the error was set, and returns nil.
func literalErrorAsBool() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	if err != nil {
		failed := err != nil
		return maybe(failed)
	}
	return nil
}

// TestLiteralErrorMatched leaks the goroutine: errors.Is fails the test
// only on one error, and any other non-nil error lets the test go on.
func TestLiteralErrorMatched(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	if errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func flagged(bool) error {
	return nil
}

// literalCompletedAsBool leaks the goroutine: it returns only
// whether the literal's error was nil, which hands no error up.
func literalCompletedAsBool() bool {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	return func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}() == nil
}

// literalCompletedBesideNilError leaks the goroutine: the call sits
// in the bool result, and the error result is nil.
func literalCompletedBesideNilError() (bool, error) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	return func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}() == nil, nil
}

// literalErrorFlagReturned leaks the goroutine: the returned call
// sees only whether the error was set, and returns nil.
func literalErrorFlagReturned() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	return flagged(err != nil)
}

// literalErrorFlagReassigned leaks the goroutine: the error is
// replaced by a call that sees only whether it was set.
func literalErrorFlagReassigned() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	err = flagged(err != nil)
	if err == nil {
		return nil
	}
	return err
}

type literalResult struct {
	err error
}

// literalErrorIndexed is an expected false positive (ENG-1004):
// the error is stored at an index,
// which the check does not follow.
func literalErrorIndexed() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	errs := make([]error, 1)
	errs[0] = func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()
	return errors.Join(errs...)
}

// literalErrorAsOk is an expected false positive (ENG-1004):
// the check reads only a bool computed
// from the error, which it does not follow.
func literalErrorAsOk() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	ok := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}() == nil
	if !ok {
		return errStop
	}
	return nil
}

// literalErrorInField is an expected false positive (ENG-1004):
// the error is stored in a struct
// field, which the check does not follow.
func literalErrorInField() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	res := literalResult{err: func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}()}
	if res.err != nil {
		return res.err
	}
	return nil
}

// deferredLiteralErrorNamed is an expected false positive (ENG-1004):
// a deferred literal assigns the
// error to the named result, which the check does not follow.
func deferredLiteralErrorNamed() (err error) {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	defer func() {
		err = func() error {
			if testing.Short() {
				return context.Canceled
			}
			wg.Wait()
			return nil
		}()
	}()
	return nil
}

// literalErrorExceptEOF leaks the goroutine: the literal may return
// io.EOF before its Wait, and the caller treats io.EOF as success.
func literalErrorExceptEOF() error {
	var wg sync.WaitGroup
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	if err := func() error {
		if testing.Short() {
			return context.Canceled
		}
		wg.Wait()
		return nil
	}(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
