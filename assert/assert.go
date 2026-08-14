// Package assert provides always-on assertions for programmer errors.
//
// Assertions are not error handling. An error is an expected operating
// condition that the caller must handle. An assertion failure means the code
// itself is wrong, and the only correct response to wrong code is to crash
// before it corrupts anything.
//
// Assertions stay enabled in production builds. They downgrade correctness
// bugs into liveness bugs, which is the trade we want. The fast path is a
// predictable branch and inlines away to almost nothing.
//
// Copy this package to internal/assert in your project so it cannot leak
// into your public API. This is the canonical, tested source.
package assert

import (
	"cmp"
	"fmt"
)

// Ok panics when cond is false.
//
// The msg argument must be a constant string so the call allocates nothing
// on the fast path. When you need formatting, guard the call yourself:
//
//	if len(entries) > EntriesMax {
//		assert.Fail("entries %d exceeds max %d", len(entries), EntriesMax)
//	}
//
// This matters because variadic arguments are boxed at the call site, before
// the callee ever tests the condition, so a formatted assert allocates on
// every call including the ones that pass.
func Ok(cond bool, msg string) {
	if !cond {
		fail(msg)
	}
}

// Implies panics when a is true and b is false.
//
// This is the Go replacement for the single-line `if (a) assert(b)` form,
// which gofmt will not let you write.
func Implies(a, b bool, msg string) {
	if a && !b {
		fail("implication violated: " + msg)
	}
}

// Equal panics when got and want differ.
func Equal[T comparable](got, want T, msg string) {
	if got != want {
		fail(fmt.Sprintf("%s: got %v, want %v", msg, got, want))
	}
}

// Between panics unless low <= value <= high.
//
// Bounds are inclusive on both ends. Say what you mean about the ends of a
// range and you remove a whole family of off-by-one bugs.
func Between[T cmp.Ordered](value, low, high T, msg string) {
	if value < low || value > high {
		fail(fmt.Sprintf("%s: %v not in [%v, %v]", msg, value, low, high))
	}
}

// Index panics unless index is a valid index into a container of length count.
func Index(index, count int, msg string) {
	if index < 0 || index >= count {
		fail(fmt.Sprintf("%s: index %d out of range [0, %d)", msg, index, count))
	}
}

// NoError panics when err is non-nil.
//
// Use this only where an error is structurally impossible, for example writing
// to a bytes.Buffer. Everywhere else, handle the error.
func NoError(err error, msg string) {
	if err != nil {
		fail(fmt.Sprintf("%s: %v", msg, err))
	}
}

// Unreachable panics unconditionally. Put it in the default arm of a switch
// over a closed set of values, so that adding a new value fails loudly instead
// of falling through silently.
func Unreachable(msg string) {
	fail("unreachable: " + msg)
}

// Fail panics with a formatted message. Call it inside an if, never as the
// whole assertion, so the formatting cost is paid only when the check fails.
func Fail(format string, args ...any) {
	fail(fmt.Sprintf(format, args...))
}

// Invariant panics when cond is false, naming the violated invariant.
//
// At run time this is Ok. The ID exists for the analyzer: TS-A07 counts the
// functions referencing each invariant, TS-A08 compares the invariant sets of
// symmetric boundary functions, and TS-A09 requires a Violates test per
// declared invariant. The ~string constraint couples by shape rather than by
// import, so this package needs no dependency on any project's inv package
// and the failure message names the invariant with no generated code.
func Invariant[ID ~string](id ID, cond bool) {
	if !cond {
		fail("invariant violated: " + string(id))
	}
}

// Violates runs fn and panics unless fn fails the named invariant. It is the
// negative-space proof TS-A09 requires: an invariant no test can violate is
// either unreachable or asserted wrong, and you want to know which.
func Violates[ID ~string](id ID, fn func()) {
	defer func() {
		switch r := recover().(type) {
		case nil:
			fail("invariant " + string(id) + " was not violated")
		case string:
			if r != "assertion failed: invariant violated: "+string(id) {
				panic(r) // some other failure; not ours to swallow
			}
		default:
			panic(r)
		}
	}()
	fn()
}

// fail is kept out of line so that its callers stay within the inlining
// budget. Everything above should compile down to a branch and a call that
// never happens.
//
//go:noinline
func fail(msg string) {
	panic("assertion failed: " + msg)
}
