// Limits that look clamped but are not: a closure that writes the limit after
// the clamp, and a branch that sets a constant no loop reaches.
package fixture

import "lib"

// SpinClampedThenClosureWrites undoes the clamp by calling a function literal
// that assigns the setting.
func SpinClampedThenClosureWrites(tuning, later Tuning, done func() bool) error {
	spins := tuning.Spins
	undo := func() { spins = later.Spins }
	spins = min(spins, spinsMax)
	undo()
	return lib.Spin(done, spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinSetHugeInBranch sets a constant no loop reaches when the setting is large.
func SpinSetHugeInBranch(tuning Tuning, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	if spins >= 100 {
		spins = 1 << 62 // want `TS-S02: this value is too large for the loop to ever reach`
	}
	return lib.Spin(done, spins)
}

// SpinClampedToHuge clamps against a maximum no loop reaches.
func SpinClampedToHuge(tuning Tuning, done func() bool) error {
	spins := tuning.Spins     // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	spins = min(spins, 1<<62) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	return lib.Spin(done, spins)
}
