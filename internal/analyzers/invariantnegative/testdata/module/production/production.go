// Package production asserts every invariant from inv at least once, so
// each const's type is recognized as an invariant type.
package production

import (
	"fixture.example/invariantnegative/assert"
	"fixture.example/invariantnegative/inv"
)

// CheckAll asserts every invariant this corpus declares.
func CheckAll(ok bool) {
	assert.Invariant(inv.NoTest, ok)
	assert.Invariant(inv.ViolatedOutsideTest, ok)
	assert.Invariant(inv.ViolatedInExternalTest, ok)
	assert.Invariant(inv.ViolatedInInternalTest, ok)
}
