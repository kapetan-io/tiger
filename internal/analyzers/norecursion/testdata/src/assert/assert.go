// Package assert is a minimal stand-in for the real assert package, present
// only so the ts-s01 compliant fixtures can call assert.Ok like real code
// would.
package assert

// Ok panics when cond is false.
func Ok(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}
