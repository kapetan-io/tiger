// Function literals count as functions: a Wait that follows the Go in the
// literal around it, or in a function around that, waits for it, whoever
// calls the literal. So does a Wait after the Go in a literal that runs before
// the call holding it returns: a literal called where it is written, a saved
// literal that is called while the variable holds it, a t.Run or b.Run body
// that does not call t.Parallel, or a parallel subtest started from one of
// these bodies on the t of a subtest that is not parallel itself or of a
// parallel subtest started from its own body.
package fixture

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"

	"golang.org/x/sync/errgroup"
)

var errStop = errors.New("stop")

func ignore(func()) {}

func TestLoopInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Go(func() { _ = i })
		}
		wg.Wait()
	})
}

func TestWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {})
		wg.Wait()
	})
}

func TestDeferredWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		defer wg.Wait()
		wg.Go(func() {})
	})
}

func TestDeferredWaitAfterGoInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {})
		defer wg.Wait()
	})
}

func TestWaitInSynctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {})
		wg.Wait()
	})
}

// TestOuterGroupInSubtest declares the group in the test function and both
// starts and waits on it inside the subtest.
func TestOuterGroupInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	t.Run("x", func(t *testing.T) {
		wg.Go(func() {})
		wg.Wait()
	})
}

// TestWaitAfterSubtest starts inside the subtest and waits after it returns.
func TestWaitAfterSubtest(t *testing.T) {
	var wg sync.WaitGroup
	t.Run("x", func(t *testing.T) {
		wg.Go(func() {})
	})
	wg.Wait()
}

// TestWaitInEnclosingLiteral starts in an inner literal and waits in the
// literal around it, after the inner one runs.
func TestWaitInEnclosingLiteral(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		t.Run("y", func(t *testing.T) {
			wg.Go(func() {})
		})
		wg.Wait()
	})
}

// TestParallelSubtestWaits waits in its own literal; compare
// TestParallelSubtestOnTestGroup in knownmiss_literal_test.go.
func TestParallelSubtestWaits(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		wg.Go(func() {})
		wg.Wait()
	})
}

// TestGroupInsideGoroutine waits on a second group inside a goroutine of the
// first, and the first is waited by the test.
func TestGroupInsideGoroutine(t *testing.T) {
	var g errgroup.Group
	g.Go(func() error {
		var wg sync.WaitGroup
		wg.Go(func() {})
		wg.Wait()
		return nil
	})
	_ = g.Wait()
}

// TestWaitAroundOwnGoroutine starts from inside a goroutine of the group and
// waits after that goroutine starts, so the Wait covers both.
func TestWaitAroundOwnGoroutine(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() {
			wg.Go(func() {})
		})
		wg.Wait()
	})
}

func TestSavedAndCalled(t *testing.T) {
	run := func() {
		var wg sync.WaitGroup
		wg.Go(func() {})
		wg.Wait()
	}
	run()
}

func TestSavedAndCalledOuterGroup(t *testing.T) {
	var wg sync.WaitGroup
	run := func() {
		wg.Go(func() {})
		wg.Wait()
	}
	run()
}

// TestSavedNeverCalled waits after the Go in the same literal, so the start
// is waited however the literal is called, or if it never is.
func TestSavedNeverCalled(t *testing.T) {
	var wg sync.WaitGroup
	run := func() {
		wg.Go(func() {})
		wg.Wait()
	}
	_ = run
}

func TestPassedAndNotCalled(t *testing.T) {
	ignore(func() {
		var wg sync.WaitGroup
		wg.Go(func() {})
		wg.Wait()
	})
}

// TestErrorBeforeWaitInLiteral returns a non-nil error before the Wait, which
// is the failure path and needs no Wait.
func TestErrorBeforeWaitInLiteral(t *testing.T) {
	run := func() error {
		var wg sync.WaitGroup
		wg.Go(func() {})
		if testing.Short() {
			return errStop
		}
		wg.Wait()
		return nil
	}
	_ = run()
}

// TestNamedErrorSetBeforeWaitInLiteral returns its named error result after
// setting it, which is the failure path and needs no Wait.
func TestNamedErrorSetBeforeWaitInLiteral(t *testing.T) {
	run := func() (err error) {
		var wg sync.WaitGroup
		wg.Go(func() {})
		if testing.Short() {
			err = errStop
			return err
		}
		wg.Wait()
		return nil
	}
	_ = run()
}

type subtestHolder struct {
	wg sync.WaitGroup
}

// runAll starts and waits on a field group inside a subtest of an unexported
// method.
func (h *subtestHolder) runAll(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		h.wg.Go(func() {})
		h.wg.Wait()
	})
}

func TestTryGoInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var g errgroup.Group
		if !g.TryGo(func() error { return nil }) {
			return
		}
		_ = g.Wait()
	})
}

func TestDeferredLiteralWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		var wg sync.WaitGroup
		defer func() { wg.Wait() }()
		wg.Go(func() {})
	})
}

// TestSelectThenWaitInSubtest waits after a select in the subtest, on every
// path.
func TestSelectThenWaitInSubtest(t *testing.T) {
	t.Run("x", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		wg.Go(func() {})
		select {
		case <-ctx.Done():
		default:
		}
		wg.Wait()
	})
}

var runPackageLiteral = func() {
	var wg sync.WaitGroup
	wg.Go(func() {})
	wg.Wait()
}

// TestWaitOnlyInLaterSubtest waits in a subtest after the Go. t.Run runs a
// literal that does not call t.Parallel before it returns, so the Wait runs.
func TestWaitOnlyInLaterSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		wg.Wait()
	})
}

// BenchmarkWaitInLaterRun is the same shape through b.Run.
func BenchmarkWaitInLaterRun(b *testing.B) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	b.Run("x", func(b *testing.B) {
		wg.Wait()
	})
}

// TestSavedWaitCalledLater saves a literal that waits and calls it after the
// Go.
func TestSavedWaitCalledLater(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {})
	wait()
}

// TestSavedWaitDeclaredWithVar is the same through a var declaration.
func TestSavedWaitDeclaredWithVar(t *testing.T) {
	var wg sync.WaitGroup
	var wait = func() { wg.Wait() }
	wg.Go(func() {})
	wait()
}

// TestSavedWaitInSubtest calls a saved waiting literal from a subtest after
// the Go.
func TestSavedWaitInSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		wait()
	})
}

// TestSavedWaitCalledOnEveryBranch calls the saved literal on both branches.
func TestSavedWaitCalledOnEveryBranch(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {})
	if testing.Short() {
		wait()
		return
	}
	wait()
}

// TestNestedSubtestWaits waits two subtests down, after the Go.
func TestNestedSubtestWaits(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		t.Run("y", func(t *testing.T) {
			wg.Wait()
		})
	})
}

// TestParallelInNestedSubtestWaits waits in the outer subtest, which runs
// before t.Run returns; only the inner subtest is parallel.
func TestParallelInNestedSubtestWaits(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		t.Run("y", func(t *testing.T) {
			t.Parallel()
		})
		wg.Wait()
	})
}

// TestSelfCallingSavedLiteral calls a saved literal that calls itself.
func TestSelfCallingSavedLiteral(t *testing.T) {
	var wg sync.WaitGroup
	var wait func(int)
	wait = func(n int) {
		if n > 0 {
			wait(n - 1)
		}
		wg.Wait()
	}
	wg.Go(func() {})
	wait(1)
}

// TestSavedSubtestWaits passes a saved subtest body to t.Run after the Go.
func TestSavedSubtestWaits(t *testing.T) {
	var wg sync.WaitGroup
	body := func(t *testing.T) { wg.Wait() }
	wg.Go(func() {})
	t.Run("x", body)
}

// TestSavedWaitWrittenAfterGo saves the waiting literal after the Go and calls
// it.
func TestSavedWaitWrittenAfterGo(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	wait := func() { wg.Wait() }
	wait()
}

// TestDeferredSavedWait defers the saved waiting literal before the Go.
func TestDeferredSavedWait(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	defer wait()
	wg.Go(func() {})
}

// TestErrgroupWaitInSubtest waits on an errgroup in a later subtest.
func TestErrgroupWaitInSubtest(t *testing.T) {
	var g errgroup.Group
	g.Go(func() error { return nil })
	t.Run("x", func(t *testing.T) {
		if err := g.Wait(); err != nil {
			t.Error(err)
		}
	})
}

// TestSubtestResultChecked waits in a subtest whose result picks the branch.
func TestSubtestResultChecked(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	if !t.Run("x", func(t *testing.T) { wg.Wait() }) {
		return
	}
}

// TestDeferredWaitInLaterSubtest defers the Wait inside a subtest after the Go.
func TestDeferredWaitInLaterSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		defer wg.Wait()
		t.Log("x")
	})
}

// TestWaitAsArgumentBeforeCall waits in an argument followed by another call
// in the same statement.
func TestWaitAsArgumentBeforeCall(t *testing.T) {
	var g errgroup.Group
	g.Go(func() error { return nil })
	_ = errors.Join(g.Wait(), errors.New("x"))
}

func runParallelCase(t *testing.T) {
	t.Run("case", func(t *testing.T) { t.Parallel() })
}

// TestHelperParallelOnOtherSubtest waits in a subtest whose helper makes a
// subtest of its own parallel, not the one it was handed.
func TestHelperParallelOnOtherSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		runParallelCase(t)
		wg.Wait()
	})
}

// TestWaitInUnnamedSubtestParams waits in subtests whose t is unnamed or
// blank, so neither can call t.Parallel.
func TestWaitInUnnamedSubtestParams(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(*testing.T) { wg.Wait() })
	t.Run("y", func(_ *testing.T) { wg.Wait() })
}

var runPackageSavedWait = func() {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {})
	wait()
}

func recurseWithSubtest(t *testing.T, n int) {
	if n == 0 {
		return
	}
	recurseWithSubtest(t, n-1)
	recurseWithSubtest(t, n-1)
	recurseWithSubtest(t, n-1)
	recurseWithSubtest(t, n-1)
	recurseWithSubtest(t, n-1)
	recurseWithSubtest(t, n-1)
	recurseWithSubtest(t, n-1)
	recurseWithSubtest(t, n-1)
}

// TestWaitAfterRecursiveHelper hands the subtest's t to a helper that passes it
// to itself eight times, and none of them calls t.Parallel.
func TestWaitAfterRecursiveHelper(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		recurseWithSubtest(t, 3)
		wg.Wait()
	})
}

type parentSuite struct {
	*testing.T
}

// TestEmbeddedParallelOnOtherTest calls Parallel through a struct that embeds
// the parent's t, not the subtest's, so the subtest still runs before t.Run
// returns.
func TestEmbeddedParallelOnOtherTest(t *testing.T) {
	var wg sync.WaitGroup
	suite := parentSuite{t}
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		if testing.Short() {
			suite.Parallel()
		}
		wg.Wait()
	})
}

// TestStartAndSubtestWaitInExpired starts and waits inside the ctx.Done()
// case, with the Wait in a subtest run there.
func TestStartAndSubtestWaitInExpired(t *testing.T) {
	var wg sync.WaitGroup
	select {
	case <-t.Context().Done():
		wg.Go(func() {})
		t.Run("x", func(t *testing.T) { wg.Wait() })
	default:
	}
}

// TestWaitInParallelSubtestOneLevelDown waits in a parallel subtest of a
// subtest. The parallel subtest runs after "x" returns, but t.Run("x") does
// not return until it finishes, so the Wait runs before the test goes on.
func TestWaitInParallelSubtestOneLevelDown(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		t.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestSavedWaitReassignedAfterCall replaces the saved literal only after it
// was called, so the call runs the literal that waits.
func TestSavedWaitReassignedAfterCall(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {})
	wait()
	wait = func() {}
	_ = wait
}

// TestWaitInParallelSubtestTwoLevelsDown waits in a parallel subtest of a
// parallel subtest of a plain subtest. t.Run("x") waits for both.
func TestWaitInParallelSubtestTwoLevelsDown(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		t.Run("y", func(t *testing.T) {
			t.Parallel()
			t.Run("z", func(t *testing.T) {
				t.Parallel()
				wg.Wait()
			})
		})
	})
}

// TestWaitInParallelSubtestFromCalledLiteral starts the parallel subtest on
// the plain subtest's t from a literal called where it is written.
func TestWaitInParallelSubtestFromCalledLiteral(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		func() {
			t.Run("y", func(t *testing.T) {
				t.Parallel()
				wg.Wait()
			})
		}()
	})
}

// TestSavedWaitAssignedLast replaces a literal that does not wait with one
// that does before the call.
func TestSavedWaitAssignedLast(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() {}
	wait = func() { wg.Wait() }
	wg.Go(func() {})
	wait()
}

// TestWaitInParallelSubtestOnAliasOfSubtest starts the parallel subtest on a
// copy of the plain subtest's t, which is the same subtest.
func TestWaitInParallelSubtestOnAliasOfSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		inner := t
		inner.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestWaitInParallelSubtestStartedFromPlainSubtest starts the parallel
// subtest on the outer subtest's t from inside a plain subtest of it.
// t.Run("x") still waits for it.
func TestWaitInParallelSubtestStartedFromPlainSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(outer *testing.T) {
		outer.Run("z", func(t *testing.T) {
			outer.Run("y", func(t *testing.T) {
				t.Parallel()
				wg.Wait()
			})
		})
	})
}

// TestWaitInDeferredParallelSubtest defers t.Run of a parallel subtest in a
// plain subtest. The deferred call starts it before "x" returns.
func TestWaitInDeferredParallelSubtest(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {})
	t.Run("x", func(t *testing.T) {
		defer t.Run("y", func(t *testing.T) {
			t.Parallel()
			wg.Wait()
		})
	})
}

// TestSavedWaitReplacementSkippedByGoto jumps over the replacement, so the
// call runs the literal that waits.
func TestSavedWaitReplacementSkippedByGoto(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	wg.Go(func() {})
	goto call
	wait = func() {}
call:
	wait()
}

// TestSavedWaitDeclaredWithoutValue declares the variable with no value,
// assigns the waiting literal, and replaces it only after the call.
func TestSavedWaitDeclaredWithoutValue(t *testing.T) {
	var wg sync.WaitGroup
	var wait func()
	wait = func() { wg.Wait() }
	wg.Go(func() {})
	wait()
	wait = func() {}
	_ = wait
}

// TestDeferredSavedWaitReplacedAfterDefer defers the saved literal while it
// waits. The defer keeps that value, so replacing it later does not matter.
func TestDeferredSavedWaitReplacedAfterDefer(t *testing.T) {
	var wg sync.WaitGroup
	wait := func() { wg.Wait() }
	defer wait()
	wait = func() {}
	_ = wait
	wg.Go(func() {})
}
