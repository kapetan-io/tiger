package methodsite_test

import (
	"testing"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/inv"
	"github.com/kapetan-io/tiger/methodsite"
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
