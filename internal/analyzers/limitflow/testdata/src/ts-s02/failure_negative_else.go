// A clamp caps the upper end only, so subtracting or negating a clamped value
// turns a large negative one into a large positive one. A rejecting if also
// leaves its else branch free to replace the value, and a parameter the
// function writes carries what it was written with.
package fixture

import (
	"errors"

	"lib"
)

// Window is a request that can carry a negative offset.
type Window struct{ Offset int }

// SpinSubtractsClamped takes a clamped offset away from a length.
func SpinSubtractsClamped(window Window, items []int, done func() bool) error {
	offset := min(window.Offset, spinsMax)
	return lib.SpinReported(done, len(items)-offset) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinNegatesClamped negates a clamped offset.
func SpinNegatesClamped(window Window, done func() bool) error {
	offset := min(window.Offset, spinsMax)
	return lib.SpinReported(done, -offset) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinReplacedInElse rejects a large setting and replaces it in the else.
func SpinReplacedInElse(tuning, later Tuning, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	if spins > spinsMax {
		return errors.New("too large")
	} else {
		spins = later.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	}
	return lib.SpinReported(done, spins)
}

// SpinParameterReassigned overwrites its limit parameter from a setting.
func SpinParameterReassigned(done func() bool, spins int, tuning Tuning) error { // want SpinParameterReassigned:`\[1\]`
	spins = tuning.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	return lib.SpinReported(done, spins)
}

// SpinExclusiveOrClamped takes the exclusive or of a clamped offset and -1,
// which turns a large negative offset into a large positive count.
func SpinExclusiveOrClamped(window Window, done func() bool) error {
	offset := min(window.Offset, spinsMax)
	return lib.SpinReported(done, offset^-1) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinProductOfClamped multiplies two clamped offsets, so two large negative
// offsets give a large positive count.
func SpinProductOfClamped(window Window, done func() bool) error {
	first := min(window.Offset, spinsMax)
	second := min(window.Offset, spinsMax)
	return lib.SpinReported(done, first*second) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinBitClearedByClamped clears the bits of a clamped offset with another,
// which can clear the sign bit.
func SpinBitClearedByClamped(window Window, done func() bool) error {
	first := min(window.Offset, spinsMax)
	second := min(window.Offset, spinsMax)
	return lib.SpinReported(done, first&^second) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinSubtractsNegativeConstant subtracts a negative constant, which adds its
// magnitude to the clamped offset.
func SpinSubtractsNegativeConstant(window Window, done func() bool) error {
	offset := min(window.Offset, spinsMax)
	return lib.SpinReported(done, offset-(-1<<40)) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinDividedByMinusOne divides a clamped offset by a negative constant, which
// flips a large negative offset to a large positive count.
func SpinDividedByMinusOne(window Window, done func() bool) error {
	offset := min(window.Offset, spinsMax)
	return lib.SpinReported(done, offset/-1) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinRemainderOfClamped takes the remainder by a clamped divisor, which can
// be a large negative number.
func SpinRemainderOfClamped(window Window, total int, done func() bool) error {
	divisor := min(window.Offset, spinsMax)
	return lib.SpinReported(done, total%divisor) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}
