// Package production asserts every compliant invariant from inv at least
// once outside a _test.go file.
package production

import (
	"fixture.example/invariantrefs/assert"
	"fixture.example/invariantrefs/inv"
)

// CheckOnce asserts AssertedOnce, its one production site.
func CheckOnce(ok bool) {
	assert.Invariant(inv.AssertedOnce, ok)
}

// CheckViaClosure asserts AssertedInClosure from a closure, proving
// closures fold into their enclosing function for TS-A07's count.
func CheckViaClosure(ok bool) {
	check := func() {
		assert.Invariant(inv.AssertedInClosure, ok)
	}
	check()
}
