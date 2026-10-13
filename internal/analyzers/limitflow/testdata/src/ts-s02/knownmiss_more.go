// A limit handed to an interface method is not traced.
package fixture

// Spinner spins up to a limit.
type Spinner interface {
	Spin(done func() bool, limit int) error
}

// ThroughInterface passes a config value to a method on an interface.
//
// known-miss: the trace follows declared functions and methods, and an
// interface method has no body to follow, so the config value is never
// judged.
func ThroughInterface(spinner Spinner, config SpinConfig, done func() bool) error {
	return spinner.Spin(done, config.Spins)
}
