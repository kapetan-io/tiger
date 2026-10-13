// More ways a limit is bounded where it enters: a rejection on either side of
// an else, a panic, >= and a flipped comparison, a rejection with another
// term, a remainder and a mask.
package fixture

import "lib"

// ClampedElse uses the value only in the branch that runs when it is small.
func ClampedElse(tuning Tuning, done func() bool) error {
	if tuning.Spins > spinsMax {
		return errTooLarge
	} else {
		return lib.SpinReported(done, tuning.Spins)
	}
}

// ClampedByPanic rejects a large value with a panic.
func ClampedByPanic(tuning Tuning, done func() bool) error {
	if tuning.Spins > spinsMax {
		panic("spins too large")
	}
	return lib.SpinReported(done, tuning.Spins)
}

// ClampedAtMost rejects the maximum itself too.
func ClampedAtMost(tuning Tuning, done func() bool) error {
	if tuning.Spins >= spinsMax {
		return errTooLarge
	}
	return lib.SpinReported(done, tuning.Spins)
}

// ClampedFlipped writes the comparison with the maximum first.
func ClampedFlipped(tuning Tuning, done func() bool) error {
	if spinsMax < tuning.Spins {
		return errTooLarge
	}
	return lib.SpinReported(done, tuning.Spins)
}

// ClampedOrOther rejects on a second term as well.
func ClampedOrOther(tuning Tuning, done func() bool) error {
	if tuning.Spins > spinsMax || done == nil {
		return errTooLarge
	}
	return lib.SpinReported(done, tuning.Spins)
}

// Wrapped takes the remainder, which is as bounded as its divisor.
func Wrapped(tuning Tuning, done func() bool) error {
	return lib.SpinReported(done, tuning.Spins%spinsMax)
}

// Masked masks with a constant.
func Masked(tuning Tuning, done func() bool) error {
	return lib.SpinReported(done, tuning.Spins&0xff)
}

// SpinClampedTimesTwo scales a clamped offset by a constant that is not
// negative, which cannot turn it into a large positive count.
func SpinClampedTimesTwo(tuning Tuning, done func() bool) error {
	spins := min(tuning.Spins, spinsMax)
	return lib.SpinReported(done, spins*2)
}

// SpinClampedExclusiveOrMask masks a clamped offset with a constant that is
// not negative.
func SpinClampedExclusiveOrMask(tuning Tuning, done func() bool) error {
	spins := min(tuning.Spins, spinsMax)
	return lib.SpinReported(done, spins^7)
}
