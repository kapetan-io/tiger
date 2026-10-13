// Checks that look like clamps and are not: a rejection that lets the value
// through, one that rejects the wrong side, one that needs a second term, and
// arithmetic that leaves a value unbounded.
package fixture

import "lib"

// RejectsNothing notices the large value and carries on.
func RejectsNothing(tuning Tuning, done func() bool) error {
	if tuning.Spins > spinsMax {
		_ = done()
	}
	return lib.SpinReported(done, tuning.Spins) // want `TS-S02: this value bounds a loop but has no upper bound where it enters`
}

// RejectsSmall rejects the small values and keeps the large ones.
func RejectsSmall(tuning Tuning, done func() bool) error {
	if tuning.Spins < spinsMax {
		return errTooLarge
	}
	return lib.SpinReported(done, tuning.Spins) // want `TS-S02: this value bounds a loop but has no upper bound where it enters`
}

// RejectsOnlyWhenFlagged rejects a large value only when another term holds.
func RejectsOnlyWhenFlagged(tuning Tuning, flagged bool, done func() bool) error {
	if tuning.Spins > spinsMax && flagged {
		return errTooLarge
	}
	return lib.SpinReported(done, tuning.Spins) // want `TS-S02: this value bounds a loop but has no upper bound where it enters`
}

// FloorOnly raises a value to a floor, which sets no ceiling.
func FloorOnly(tuning Tuning, done func() bool) error {
	return lib.SpinReported(done, max(tuning.Spins, 1)) // want `TS-S02: this value bounds a loop but has no upper bound where it enters`
}

// OneMore adds to a value from outside.
func OneMore(tuning Tuning, done func() bool) error {
	return lib.SpinReported(done, tuning.Spins+1) // want `TS-S02: this value bounds a loop but has no upper bound where it enters`
}
