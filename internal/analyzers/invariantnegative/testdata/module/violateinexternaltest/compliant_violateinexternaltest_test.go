package violateinexternaltest_test

import (
	"testing"

	"fixture.example/invariantnegative/assert"
	"fixture.example/invariantnegative/inv"
)

// TestViolatesFromExternalPackage violates ViolatedInExternalTest from an
// external test package (foo_test) — TS-A09's compliant external-test
// shape.
func TestViolatesFromExternalPackage(t *testing.T) {
	assert.Violates(inv.ViolatedInExternalTest, func() {
		assert.Invariant(inv.ViolatedInExternalTest, false)
	})
}
