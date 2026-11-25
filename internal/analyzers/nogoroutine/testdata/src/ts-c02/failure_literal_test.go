// TS-C02 findings inside function literals: no Wait follows the Go on every
// success path of the literal or a function around it, the Wait runs inside
// the group's own goroutine, or the Wait sits in a literal that may not run
// before the call holding it returns (saved and never called, replaced on
// some path before the call, passed to another function, a parallel subtest
// of the function holding the Go, or t.Cleanup).
package fixture

import (
	"context"
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

// TestSavedWaitReplacedBeforeNextPass replaces the saved literal after the
// call, but the loop comes back to the call, which may then run the literal
// that does not wait.
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

// TestSavedWaitReplacedInCalledLiteral replaces the saved literal from a
// literal called before the saved one is.
func TestSavedWaitReplacedInCalledLiteral(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	func() { wait = func() {} }()
	wg.Go(func() {}) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
	wait()
	wait = func() {}
}
