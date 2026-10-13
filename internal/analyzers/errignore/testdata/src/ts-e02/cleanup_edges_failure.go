// Neighbours of the cleanup exemption that must still fire.
package fixture

import (
	"os"
	"strconv"
	"testing"
)

func hashValidation(raw string) int {
	n, _ := strconv. // want `TS-E02: this error is discarded with _`
				Atoi(raw)
	return n
}

func hashValidationInTest(t *testing.T, raw string) int {
	n, _ := strconv. // want `TS-E02: this error is discarded with _`
				Atoi(raw)
	return n
}

// The statement after a defer in the same function is not exempt.
func afterDefer(f *os.File) {
	defer func() { _ = f.Close() }()
	_ = f. // want `TS-E02: this error is discarded with _`
		Close()
}

// Code in a Cleanup-registering function outside the Cleanup literal.
func besideCleanup(t *testing.T, f *os.File) {
	t.Cleanup(func() { _ = f.Close() })
	_, _ = f. // want `TS-E02: this error is discarded with _`
			Stat()
}

// A literal built outside the call and passed by name is not inside it.
func literalByName(t *testing.T, f *os.File) {
	closer := func() {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
	}
	t.Cleanup(closer)
}

// A Cleanup method value called through a variable is not recognised as
// testing's Cleanup.
func methodValueCleanup(t *testing.T, f *os.File) {
	register := t.Cleanup
	register(func() {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
	})
}

// A subtest body is not a cleanup.
func subtestBody(t *testing.T, f *os.File) {
	t.Run("sub", func(t *testing.T) {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
	})
}

// A goroutine body is not a cleanup.
func goroutineBody(f *os.File) {
	go func() {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
	}()
}
