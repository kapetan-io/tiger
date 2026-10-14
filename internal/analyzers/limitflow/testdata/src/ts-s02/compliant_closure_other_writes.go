// A function literal that writes something other than the limit leaves the
// clamp standing.
package fixture

import "lib"

// SpinClampedClosureWritesOther clamps the setting while a function literal
// writes a different variable.
func SpinClampedClosureWritesOther(tuning, later Tuning, done func() bool) error {
	spins := tuning.Spins
	ceiling := 0
	note := func() { ceiling = later.Spins }
	spins = min(spins, spinsMax)
	note()
	_ = ceiling
	return lib.Spin(done, spins)
}

// SpinClampedClosureReads clamps the setting while a function literal only
// reads it.
func SpinClampedClosureReads(tuning Tuning, done func() bool) error {
	spins := min(tuning.Spins, spinsMax)
	look := func() int { return spins }
	_ = look()
	return lib.Spin(done, spins)
}

// SpinFieldClampedClosureWritesOtherField clamps one field while a function
// literal writes another field.
func SpinFieldClampedClosureWritesOtherField(tuning, later Tuning, done func() bool) error {
	note := func() { tuning.Ceiling = later.Ceiling }
	tuning.Spins = min(tuning.Spins, spinsMax)
	note()
	return lib.Spin(done, tuning.Spins)
}

// SpinClampedInsideTheLiteral clamps and uses the setting inside the one
// literal that writes it, in order.
func SpinClampedInsideTheLiteral(spins int, done func() bool) error {
	run := func() error {
		spins = min(spins, spinsMax)
		return lib.Spin(done, spins)
	}
	return run()
}
