// A function that only fails the test is a call that returns to its caller,
// not a Wait, so it excuses no group it can reach. These test-support types
// sit in a file that is not a test, where a field or package-level group can
// be waited by any function that reaches it.
package fixture

import (
	"sync"
	"testing"
)

type fatalHarness struct {
	group sync.WaitGroup
	tb    testing.TB
}

func (h *fatalHarness) Start() {
	h.group.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

func (h *fatalHarness) Fail(msg string) { h.tb.Fatal(msg) }

var fatalBackground sync.WaitGroup

func StartFatalBackground() {
	fatalBackground.Go(work) // want `TS-C02: this Go call starts a goroutine, but no Wait on the group is reached`
}

func MustFail(tb testing.TB) { tb.Fatalf("fail") }
