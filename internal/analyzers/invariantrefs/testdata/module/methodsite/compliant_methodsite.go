// Package methodsite asserts AssertedInMethod from a method, proving
// TS-A07 counts method bodies as functions.
package methodsite

import (
	"fixture.example/invariantrefs/assert"
	"fixture.example/invariantrefs/inv"
)

// Checker asserts its invariant from a method body.
type Checker struct{}

// Check asserts AssertedInMethod.
func (Checker) Check(ok bool) {
	assert.Invariant(inv.AssertedInMethod, ok)
}
