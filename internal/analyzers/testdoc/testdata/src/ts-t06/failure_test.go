// The TS-T06 failure mode: a Test function with no doc comment at all.
package fixture

import "testing"

func TestNoDocComment(t *testing.T) { // want `TS-T06: TestNoDocComment has no doc comment`
	_ = t
}
