package fixture

import "testing"

// TestMain here does not call goleak, so it is not a leak check.
func TestMain(m *testing.M) { m.Run() }
