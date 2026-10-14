package fixture_test

import (
	"os"
	"testing"

	leaks "go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	leaks.VerifyTestMain(m)
	os.Exit(0)
}
