// Package assert is a corpus miniature of the always-on assertion package.
// It exists so TS-S07 and TS-S08 can resolve assert.Ok, assert.Invariant,
// and assert.Unreachable calls by their real package name and shape.
package assert

// Ok panics when cond is false.
func Ok(cond bool, msg string) {
	if !cond {
		panic("assertion failed: " + msg)
	}
}

// Invariant panics when cond is false, naming the violated invariant.
func Invariant[ID ~string](id ID, cond bool) {
	if !cond {
		panic("invariant violated: " + string(id))
	}
}

// Unreachable panics unconditionally.
func Unreachable(msg string) {
	panic("unreachable: " + msg)
}
