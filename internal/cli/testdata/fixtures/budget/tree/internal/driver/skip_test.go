package driver_test

import "testing"

// TestDeferred is deferred until the fake clock lands.
//
// Goal: exercise the fake clock once it exists.
func TestDeferred(t *testing.T) {
	t.Skip("needs the fake clock")
}
