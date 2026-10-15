// More limits that look clamped but are not: a closure that writes a field
// after the clamp, a huge constant on the large side of a guard, and a huge
// constant in an else branch.
package fixture

import "lib"

// SpinFieldClampedThenClosureWrites undoes a field clamp with a function
// literal that assigns the field.
func SpinFieldClampedThenClosureWrites(tuning, later Tuning, done func() bool) error {
	undo := func() { tuning.Spins = later.Spins }
	tuning.Spins = min(tuning.Spins, spinsMax)
	undo()
	return lib.Spin(done, tuning.Spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinGuardedByHugeMaximum only runs while the setting is under a maximum no
// loop reaches.
func SpinGuardedByHugeMaximum(tuning Tuning, done func() bool) error {
	if tuning.Spins < 1<<62 {
		return lib.Spin(done, tuning.Spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	}
	return nil
}

// SpinSetHugeInElse sets a constant no loop reaches when the setting is small.
func SpinSetHugeInElse(tuning Tuning, done func() bool) error {
	spins := tuning.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	if spins >= 100 {
		spins = spinsMax
	} else {
		spins = 1 << 62 // want `TS-S02: this value is too large for the loop to ever reach`
	}
	return lib.Spin(done, spins)
}

// SpinPassedFromLiteralAfterOtherLiteral clamps the setting outside both
// literals, and the second literal replaces it before the first uses it.
func SpinPassedFromLiteralAfterOtherLiteral(tuning, later Tuning, done func() bool) error {
	spins := tuning.Spins
	spins = min(spins, spinsMax)
	use := func() error { return lib.Spin(done, spins) } // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	replace := func() { spins = later.Spins }
	replace()
	return use()
}

// SpinPassedThenRewrittenBySecondCall uses the setting in a literal that
// rewrites it afterwards, so the second call spins on the rewritten value.
func SpinPassedThenRewrittenBySecondCall(tuning, later Tuning, done func() bool) error {
	spins := min(tuning.Spins, spinsMax)
	run := func() error {
		err := lib.Spin(done, spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
		spins = later.Spins
		return err
	}
	_ = run()
	return run()
}
