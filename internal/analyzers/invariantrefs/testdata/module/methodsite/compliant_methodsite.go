// Package methodsite asserts AssertedInMethod from a method, proving
// TS-A07 counts method bodies as functions.
package methodsite

import (
	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/inv"
)

// Checker asserts its invariant from a method body.
type Checker struct{}

// Check asserts AssertedInMethod.
func (Checker) Check(ok bool) {
	assert.Invariant(inv.AssertedInMethod, ok)
}
