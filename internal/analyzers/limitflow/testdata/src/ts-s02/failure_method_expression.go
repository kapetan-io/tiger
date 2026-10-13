// A method expression passes the receiver as its first argument, which is no
// parameter of the method.
package fixture

import "lib"

// pacer spins for a limit it is handed.
type pacer struct{}

func (pacer) Spin(done func() bool, limit int) error { // want Spin:`\[1\]`
	return lib.SpinReported(done, limit)
}

// SpinThroughMethodExpression names the method and passes the receiver first.
func SpinThroughMethodExpression(config SpinConfig, done func() bool) error {
	return pacer.Spin(pacer{}, done, config.Spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinFromDecodedRequest reads a limit from a struct decoded into a type
// with no name, which is still a value from outside.
func SpinFromDecodedRequest(done func() bool) error {
	var request struct{ Limit int }
	return lib.SpinReported(done, request.Limit) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}
