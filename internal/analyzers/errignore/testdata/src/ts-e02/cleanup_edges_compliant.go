// Further shapes inside the cleanup exemption (ADR-0017).
package fixture

import (
	"os"
	"testing"
)

type harness struct{ *testing.T }

func embeddedCleanup(h harness, f *os.File) {
	h.Cleanup(func() { _ = f.Close() })
}

func nestedInCleanup(t *testing.T, f *os.File) {
	t.Cleanup(func() {
		func() { _ = f.Close() }()
	})
}

func nestedInDefer(f *os.File) {
	defer func() {
		func() { _ = f.Close() }()
	}()
}

func goInDefer(f *os.File) {
	defer func() {
		go func() { _ = f.Close() }()
	}()
}

func deferInsideCleanup(t *testing.T, f *os.File) {
	t.Cleanup(func() {
		defer func() { _ = f.Close() }()
	})
}

func deferInsideSubtest(t *testing.T, f *os.File) {
	t.Run("sub", func(t *testing.T) {
		defer func() { _ = f.Close() }()
	})
}

func cleanupInsideSubtest(t *testing.T, f *os.File) {
	t.Run("sub", func(t *testing.T) {
		t.Cleanup(func() { _ = f.Close() })
	})
}

func multiDiscardInDefer(f *os.File) {
	defer func() {
		_, _ = f.Stat()
		_ = f.Close()
	}()
}
