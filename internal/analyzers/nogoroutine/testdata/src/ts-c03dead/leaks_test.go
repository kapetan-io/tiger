package fixture

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	if false {
		goleak.VerifyTestMain(m)
	}
	m.Run()
}

func TestRun(t *testing.T) { run() }
