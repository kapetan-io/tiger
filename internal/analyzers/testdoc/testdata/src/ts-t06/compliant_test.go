// The compliant rewrites: every Test function has a doc comment. A Goal
// line is welcome but not required.
package fixture

import "testing"

// TestWithGoal proves the passing path stays silent.
//
// Goal: a doc comment with a Goal line does not fire.
func TestWithGoal(t *testing.T) {
	_ = t
}

// TestWithoutGoal has a doc comment describing its purpose but omits the
// Goal token — this is compliant under the relaxed rule.
func TestWithoutGoal(t *testing.T) {
	_ = t
}

// Test is the exact-name edge case; the bare name "Test" is also
// recognized and must carry a doc comment.
func Test(t *testing.T) {
	_ = t
}
