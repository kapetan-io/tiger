// Package assert is the fixture's copy of tiger's assert package. The
// fixture module takes tiger's module path, so this directory has the import
// path tiger recognizes.
package assert

// Invariant panics unless cond holds, naming the invariant.
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
