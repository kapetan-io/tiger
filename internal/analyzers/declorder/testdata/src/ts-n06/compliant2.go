// TestSomething's only call to setupFixture makes it the single caller, but
// a test-function caller is exempt: this stays silent even though the name
// carries no prefix from TestSomething.
package fixture

import "testing"

func TestSomething(t *testing.T) {
	setupFixture(t)
}

func setupFixture(t *testing.T) {
	t.Helper()
}
