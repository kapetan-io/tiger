// The compliant rewrites: every Test function's doc comment carries a
// Goal line.
package fixture

import "testing"

// TestWithGoal proves the passing path stays silent.
//
// Goal: a doc comment with a Goal line does not fire.
func TestWithGoal(t *testing.T) {
	_ = t
}

// Test is the exact-name edge case; the bare name "Test" is also
// recognized and must carry a Goal line.
//
// Goal: the exact name "Test" is checked the same as any TestXxx name.
func Test(t *testing.T) {
	_ = t
}
