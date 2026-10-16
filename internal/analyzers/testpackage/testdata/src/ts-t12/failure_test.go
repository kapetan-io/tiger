// The TS-T12 failure mode: a test file declared in the package under test.
package fixture // want `TS-T12: this test file declares package fixture`

import "testing"

func TestInternal(t *testing.T) { _ = t }
