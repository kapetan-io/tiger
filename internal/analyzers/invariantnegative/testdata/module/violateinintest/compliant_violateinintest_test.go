package violateinintest

import (
	"testing"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/inv"
)

// TestViolatesFromInPackageTest violates ViolatedInInternalTest from an
// in-package _test.go file — TS-A09's compliant internal-test shape.
func TestViolatesFromInPackageTest(t *testing.T) {
	assert.Violates(inv.ViolatedInInternalTest, func() {
		assert.Invariant(inv.ViolatedInInternalTest, false)
	})
}
