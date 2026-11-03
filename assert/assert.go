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
// Import it as github.com/kapetan-io/tiger/assert. Tiger recognizes the
// package by that import path alone, so a copy under another path is
// treated as an ordinary package.
package assert

import (
	"cmp"
	"fmt"
)

func Ok(cond bool, msg string) {
	if !cond {
		fail(msg)
	}
}

func Implies(a, b bool, msg string) {
	if a && !b {
		fail("implication violated: " + msg)
	}
}

func Equal[T comparable](got, want T, msg string) {
	if got != want {
		fail(fmt.Sprintf("%s: got %v, want %v", msg, got, want))
	}
}

func Between[T cmp.Ordered](value, low, high T, msg string) {
	if value < low || value > high {
		fail(fmt.Sprintf("%s: %v not in [%v, %v]", msg, value, low, high))
	}
}

func Index(index, count int, msg string) {
	if index < 0 || index >= count {
		fail(fmt.Sprintf("%s: index %d out of range [0, %d)", msg, index, count))
	}
}

func NoError(err error, msg string) {
	if err != nil {
		fail(fmt.Sprintf("%s: %v", msg, err))
	}
}

func Unreachable(msg string) {
	fail("unreachable: " + msg)
}

func Fail(format string, args ...any) {
	fail(fmt.Sprintf(format, args...))
}

func Invariant[ID ~string](id ID, cond bool) {
	if !cond {
		fail("invariant violated: " + string(id))
	}
}

func Violates[ID ~string](id ID, fn func()) {
	defer func() {
		switch r := recover().(type) {
		case nil:
			fail("invariant " + string(id) + " was not violated")
		case string:
			if r != "assertion failed: invariant violated: "+string(id) {
				panic(r)
			}
		default:
			panic(r)
		}
	}()
	fn()
}

//go:noinline
func fail(msg string) {
	panic("assertion failed: " + msg)
}
