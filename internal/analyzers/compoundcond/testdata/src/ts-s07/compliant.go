// The TS-S07 compliant rewrites: split assertions, plus the documented ||
// exemption.
package fixture

import "assert"

// checkPairSplitCompliant splits the pair into two assertions so a failure
// names which half broke.
func checkPairSplitCompliant(a, b bool) {
	assert.Ok(a, "a")
	assert.Ok(b, "b")
}

// checkEitherCompliant uses || instead of &&: an either/or condition is not
// splittable into two independent assertions the way && is, so it stays
// silent.
func checkEitherCompliant(a, b bool) {
	assert.Ok(a || b, "a or b")
}

// checkInvariantSplitCompliant splits the invariant's paired check.
func checkInvariantSplitCompliant(id string, a, b bool) {
	assert.Invariant(id, a)
	assert.Invariant(id, b)
}

// checkSingleCompliant has nothing to split.
func checkSingleCompliant(a bool) {
	assert.Ok(a, "a")
}
