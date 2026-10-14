package fixture

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { m.Run() }

func TestRun(t *testing.T) {
	defer goleak.VerifyNone(t)
	run()
}
