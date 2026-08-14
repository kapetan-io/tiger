// The TS-D07 reported forms: every skip on a testing receiver surfaces,
// across each Skip spelling and each testing receiver type. There is no
// silencing form — the advisory stands until the skip is removed.
package fixture

import "testing"

// TestSkip covers the plain Skip spelling.
func TestSkip(t *testing.T) {
	t.Skip() // want `TS-D07: skipped test`
}

// TestSkipf covers the formatted spelling.
func TestSkipf(t *testing.T) {
	t.Skipf("flaky on windows") // want `TS-D07: skipped test`
}

// TestSkipNow covers the SkipNow spelling.
func TestSkipNow(t *testing.T) {
	t.SkipNow() // want `TS-D07: skipped test`
}

// BenchmarkSkip covers the *testing.B receiver.
func BenchmarkSkip(b *testing.B) {
	b.Skip() // want `TS-D07: skipped test`
}

// skipViaHelper covers the testing.TB interface receiver.
func skipViaHelper(tb testing.TB) {
	tb.Skip() // want `TS-D07: skipped test`
}
