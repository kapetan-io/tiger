// A value clamped by assigning min of itself and the maximum is bounded.
package fixture

import "lib"

// SpinSelfClamped clamps the setting in place before it reaches the loop.
func SpinSelfClamped(tuning Tuning, done func() bool) error {
	spins := tuning.Spins
	spins = min(spins, spinsMax)
	return lib.SpinReported(done, spins)
}
