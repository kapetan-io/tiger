// Discards in the immediately evaluated arguments of defer or Cleanup are
// not deferred cleanup. The analyzer still reports them.
package fixture

import (
	"os"
	"testing"
)

func consume(n int) {}

// A discard inside an argument of a deferred non-literal call runs
// immediately, at defer time, so it is not deferred cleanup.
func deferArgDiscard(f *os.File) {
	defer consume(func() int {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
		return 1
	}())
}

// Same for an argument of Cleanup that is evaluated immediately.
func cleanupArgDiscard(t *testing.T, f *os.File) {
	t.Cleanup(wrap(func() int {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
		return 1
	}()))
}

func wrap(n int) func() { return func() {} }
