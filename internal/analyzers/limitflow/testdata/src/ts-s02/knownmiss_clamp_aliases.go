// Documented gaps in the clamp check: a write the check cannot see because it
// happens through a name the use does not mention.
package fixture

import "lib"

// SpinSetThroughAPointer clamps a setting, then writes it through a pointer.
//
// known-miss: the check looks for writes that name the variable or what a
// pointer to it points at, and the pointer here is a second name for the
// variable.
func SpinSetThroughAPointer(tuning, later Tuning, done func() bool) error {
	spins := tuning.Spins
	if spins > spinsMax {
		return nil
	}
	alias := &spins
	*alias = later.Spins
	return lib.SpinReported(done, spins)
}
