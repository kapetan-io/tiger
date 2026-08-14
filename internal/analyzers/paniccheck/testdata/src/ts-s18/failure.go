// The TS-S18 failure mode: a naked panic invents a second crash path with
// its own message format.
package fixture

// applyEntry crashes on a malformed entry without going through assert.
func applyEntry(size int) {
	if size < 0 {
		panic("negative size") // want `TS-S18: naked panic`
	}
}

// unreachableArm hand-rolls what assert.Unreachable already is.
func unreachableArm(kind int) string {
	switch kind {
	case 0:
		return "zero"
	default:
		panic("unknown kind") // want `TS-S18: naked panic`
	}
}
