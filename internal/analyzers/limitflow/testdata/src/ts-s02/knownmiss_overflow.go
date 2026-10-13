// A clamp caps the top of a value only, and the trace does not model integer
// wrap-around at the bottom of the type.
package fixture

import "lib"

// SpinClampedTimesTwo scales a clamped setting by a constant.
//
// known-miss: a setting below -(1<<62) wraps to a large positive count when
// doubled. Catching it needs a lower bound on every clamped value, which tiger
// does not track.
func SpinClampedTimesTwo(tuning Tuning, done func() bool) error {
	spins := min(tuning.Spins, spinsMax)
	return lib.SpinReported(done, spins*2)
}

// SpinClampedSum adds two clamped settings.
//
// known-miss: two settings near the bottom of the type wrap to a large
// positive count when added, for the same reason.
func SpinClampedSum(first, second Tuning, done func() bool) error {
	sum := min(first.Spins, spinsMax) + min(second.Spins, spinsMax)
	return lib.SpinReported(done, sum)
}
