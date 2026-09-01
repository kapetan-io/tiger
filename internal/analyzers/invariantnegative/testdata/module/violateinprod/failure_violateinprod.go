// Package violateinprod calls assert.Violates from a production function,
// not a _test.go file — TS-A09 does not count it, since the guarantee is
// that a test runner actually executes the negative-space proof.
package violateinprod

import (
	"fixture.example/invariantnegative/assert"
	"fixture.example/invariantnegative/inv"
)

// RunFromProduction calls assert.Violates outside any test file.
func RunFromProduction() {
	assert.Violates(inv.ViolatedOutsideTest, func() {
		assert.Invariant(inv.ViolatedOutsideTest, false)
	})
}
