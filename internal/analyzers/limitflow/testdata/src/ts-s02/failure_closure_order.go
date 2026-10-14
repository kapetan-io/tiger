// Documented false positives of the closure check: it ignores the order a function
// literal runs in, so compliant code that has one write the limit is flagged.
package fixture

import "lib"

// SpinResetBeforeClamp has a literal write the setting before the clamp.
//
// conservative: the check cannot tell that the literal runs before the clamp,
// so the clamp is rejected although it holds.
func SpinResetBeforeClamp(tuning Tuning, done func() bool) error {
	spins := tuning.Spins
	reset := func() { spins = 5 }
	reset()
	spins = min(spins, spinsMax)
	return lib.Spin(done, spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinClampedByLiteral clamps the setting inside a literal and calls it.
//
// conservative: the literal's write is a write the clamp check cannot order, so
// the clamp it makes is rejected.
func SpinClampedByLiteral(tuning Tuning, done func() bool) error {
	spins := tuning.Spins
	clamp := func() { spins = min(spins, spinsMax) }
	clamp()
	return lib.Spin(done, spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinClampedAndUsedInLiteral clamps and uses the setting inside the one
// literal that writes it.
//
// conservative: a literal can run more than once, and the check does not tell
// which run a write belongs to, so the parameter is judged unbounded.
func SpinClampedAndUsedInLiteral(spins int, done func() bool) error { // want SpinClampedAndUsedInLiteral:`\[0\]`
	run := func() error {
		spins = min(spins, spinsMax)
		return lib.Spin(done, spins)
	}
	return run()
}
