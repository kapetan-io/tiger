// Package skipped carries one deferred test, which surfaces as a standing
// advisory on every run.
package skipped

import "testing"

// TestDeferred is deferred until the fake clock lands.
//
// Goal: exercise the fake clock once it exists.
func TestDeferred(t *testing.T) {
	t.Skip("needs the fake clock")
}
