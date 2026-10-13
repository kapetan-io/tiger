// Package assert is a corpus miniature of the always-on assertion package.
// A panic inside the assert package is the sanctioned crash path, so
// nothing here fires.
package assert

// Ok panics when cond is false.
func Ok(cond bool, msg string) {
	if !cond {
		panic("assertion failed: " + msg)
	}
}

// Unreachable panics unconditionally.
func Unreachable(msg string) {
	panic("assertion failed: unreachable: " + msg)
}
