// The compliant rewrite: the external test package reaches only exported names.
package fixture_test

import "testing"

func TestExternal(t *testing.T) { _ = t }
