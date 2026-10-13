// More TS-S02 limits that enter unbounded: a constant at half the type's
// maximum, a clamp against a value rather than a declared maximum, and a
// clamp on only one path to the loop.
package fixture

import "lib"

// Tuning holds two settings read from a file at startup.
type Tuning struct{ Spins, Ceiling int }

// SpinHalf passes the smallest constant the rule calls too large for int.
func SpinHalf(done func() bool) error {
	return lib.Spin(done, 1<<62) // want `TS-S02: this value is too large for the loop to ever reach`
}

// ClampToValue clamps one setting against another, neither of them declared.
func ClampToValue(tuning Tuning, done func() bool) error {
	return lib.SpinReported(done, min(tuning.Spins, tuning.Ceiling)) // want `TS-S02: this value bounds a loop but has no upper bound where it enters` `TS-S02: this value bounds a loop but has no upper bound where it enters`
}

// ClampOnOnePath clamps only when flagged, so the other path reaches the loop
// unbounded.
func ClampOnOnePath(tuning Tuning, flagged bool, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value bounds a loop but has no upper bound where it enters`
	if flagged {
		spins = spinsMax
	}
	return lib.SpinReported(done, spins)
}
