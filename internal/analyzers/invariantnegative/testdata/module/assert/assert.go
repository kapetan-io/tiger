// Package assert is the module's local copy of the always-on assertion
// vocabulary, matched by shape (package name assert, function names
// Invariant and Violates) rather than by import path.
package assert

// Invariant panics when cond is false, naming id.
func Invariant[ID ~string](id ID, cond bool) {
	if !cond {
		panic("assertion failed: invariant violated: " + string(id))
	}
}

// Violates runs fn and panics unless fn fails the named invariant.
func Violates[ID ~string](id ID, fn func()) {
	defer func() {
		if recover() == nil {
			panic("assertion failed: invariant " + string(id) + " was not violated")
		}
	}()
	fn()
}
