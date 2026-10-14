package fixture

import (
	"os"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	_ = goleak.VerifyTestMain
	os.Exit(m.Run())
}
