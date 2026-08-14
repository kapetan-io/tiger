// The compliant forms: tests that run stay silent, and Skip methods on
// non-testing receivers are not testing skips.
package fixture

import "testing"

// TestRuns exercises behavior without skipping.
func TestRuns(t *testing.T) {
	if unreached() {
		t.Fatal("unreached")
	}
}

// cursor is a domain type whose Skip method advances past an entry; it has
// nothing to do with skipping tests.
type cursor struct {
	offset int
}

func (c *cursor) Skip() {
	c.offset++
}

// TestDomainSkipMethod calls a Skip method on a non-testing receiver.
func TestDomainSkipMethod(t *testing.T) {
	position := cursor{}
	position.Skip()
	if position.offset != 1 {
		t.Fatal("cursor did not advance")
	}
}

func unreached() bool { return false }
