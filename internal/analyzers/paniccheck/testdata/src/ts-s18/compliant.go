// The compliant rewrite routes every crash through the assert package, and
// a shadowed identifier named panic is not the builtin.
package fixture

import "assert"

// applyEntryCompliant asserts the precondition instead of panicking.
func applyEntryCompliant(size int) {
	assert.Ok(size >= 0, "entry size is negative")
}

// unreachableArmCompliant uses the one crash path for impossible arms.
func unreachableArmCompliant(kind int) string {
	switch kind {
	case 0:
		return "zero"
	default:
		assert.Unreachable("unknown kind")
		return ""
	}
}

// shadowedPanic calls a local function that happens to be named panic; the
// builtin is not involved, so TS-S18 stays silent.
func shadowedPanic() {
	panic := func(msg string) string { return msg }
	_ = panic("not the builtin")
}
