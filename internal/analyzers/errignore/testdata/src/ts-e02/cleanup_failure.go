// Neighbours of the cleanup exemption that still fire: non-deferred
// cleanup, the discard in a function literal that is not passed to
// Cleanup, and a Cleanup method on a type from outside testing.
package fixture

import "os"

func bodyCloseNotDeferred(f *os.File) {
	_ = f. // want `TS-E02: this error is discarded with _`
		Close()
}

func literalNotCleanup(f *os.File) {
	run(func() {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
	})
}

func run(fn func()) { fn() }

type registry struct{}

func (registry) Cleanup(fn func()) { fn() }

func ownCleanup(r registry, f *os.File) {
	r.Cleanup(func() {
		_ = f. // want `TS-E02: this error is discarded with _`
			Close()
	})
}
