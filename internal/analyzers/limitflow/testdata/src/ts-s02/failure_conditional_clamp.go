// A clamp that runs on one path leaves the other path unclamped, and the
// finding lands on the unclamped write, not on the clamp.
package fixture

import "lib"

// SpinConditionallyClamped clamps the setting only when flag is set.
func SpinConditionallyClamped(tuning Tuning, flag bool, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value bounds a loop but has no upper bound where it enters`
	if flag {
		spins = min(spins, spinsMax)
	}
	return lib.SpinReported(done, spins)
}
