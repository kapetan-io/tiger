package violateinintest

import (
	"testing"

	"fixture.example/invariantnegative/assert"
	"fixture.example/invariantnegative/inv"
)

// TestViolatesFromInPackageTest violates ViolatedInInternalTest from an
// in-package _test.go file — TS-A09's compliant internal-test shape.
func TestViolatesFromInPackageTest(t *testing.T) {
	assert.Violates(inv.ViolatedInInternalTest, func() {
		assert.Invariant(inv.ViolatedInInternalTest, false)
	})
}
