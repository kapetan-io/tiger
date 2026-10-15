package fixture

import "testing"

func TestDrain(t *testing.T) {
	close(drainStop)
}
