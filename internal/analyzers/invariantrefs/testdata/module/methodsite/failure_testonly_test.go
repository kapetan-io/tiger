package methodsite_test

import (
	"testing"

	"fixture.example/invariantrefs/assert"
	"fixture.example/invariantrefs/inv"
	"fixture.example/invariantrefs/methodsite"
)

// TestCheckPasses exercises the compliant method-site assertion.
//
// Goal: a passing invariant assertion inside a method is silent.
func TestCheckPasses(t *testing.T) {
	methodsite.Checker{}.Check(true)
}

// TestTestOnlyViolatesFromTestFile is TestOnly's sole assertion site — a
// _test.go file, which TS-A07 does not count, so the failure fires despite
// this call.
func TestTestOnlyViolatesFromTestFile(t *testing.T) {
	assert.Violates(inv.TestOnly, func() {
		assert.Invariant(inv.TestOnly, false)
	})
}
