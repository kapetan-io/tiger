// Discards inside a defer statement or a testing Cleanup literal are exempt
// (ADR-0017).
package fixture

import (
	"os"
	"testing"
)

func deferredLiteral(f *os.File) {
	defer func() { _ = f.Close() }()
}

func deferredDirect(f *os.File) {
	defer func() {
		_, _ = f.Stat()
	}()
}

func testCleanup(t *testing.T, f *os.File) {
	t.Cleanup(func() { _ = f.Close() })
}

func testBCleanup(b *testing.B, f *os.File) {
	b.Cleanup(func() { _ = f.Close() })
}

func tbCleanup(tb testing.TB, f *os.File) {
	tb.Cleanup(func() { _ = f.Close() })
}
