// A clamp only counts for the value it clamped: a later write, a clamp that
// does not bound, and a shadow with the same spelling all leave the limit
// unbounded.
package fixture

import "lib"

// SpinClampedThenReassigned writes the setting again after the check.
func SpinClampedThenReassigned(tuning, later Tuning, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	if spins > spinsMax {
		return nil
	}
	spins = later.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	return lib.SpinReported(done, spins)
}

// SpinClampedByDoubling answers a too-large setting by making it larger.
func SpinClampedByDoubling(tuning Tuning, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	if spins > spinsMax {
		spins = spins * 2 // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	}
	return lib.SpinReported(done, spins)
}

// SpinClampedOtherVariable checks one variable and loops on another with the
// same name.
func SpinClampedOtherVariable(tuning, other Tuning, done func() bool) error {
	spins := tuning.Spins
	if spins > spinsMax {
		return nil
	}
	{
		spins := other.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
		return lib.SpinReported(done, spins)
	}
}

// SpinGuardedThenReplaced only runs while the setting is under the maximum,
// then replaces the setting before the call.
func SpinGuardedThenReplaced(tuning, later Tuning, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	if spins < spinsMax {
		spins = later.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
		return lib.SpinReported(done, spins)
	}
	return nil
}

// SpinCheckedAsAByte rejects a value that does not fit a byte after the byte
// is cut from it, which says nothing about the value.
func SpinCheckedAsAByte(tuning Tuning, done func() bool) error {
	if int8(tuning.Spins) > 10 {
		return nil
	}
	return lib.SpinReported(done, tuning.Spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}
