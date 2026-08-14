// The TS-T06 failure modes: a Test function with no doc comment at all,
// and one whose doc comment never states a Goal.
package fixture

import "testing"

func TestNoDocComment(t *testing.T) { // want `TS-T06: TestNoDocComment has no doc comment`
	_ = t
}

// TestDocWithoutGoal describes the setup but never says what the test
// proves.
func TestDocWithoutGoal(t *testing.T) { // want `TS-T06: TestDocWithoutGoal's doc comment has no "Goal:" line`
	_ = t
}
