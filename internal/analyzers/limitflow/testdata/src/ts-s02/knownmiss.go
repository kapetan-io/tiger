// Documented TS-S02 limit gaps: a limit that reaches a loop through a
// function value, and a constant below half the type's maximum.
package fixture

import "lib"

// ThroughValue calls the loop through a function value.
//
// known-miss: the trace follows declared functions and methods, so a call
// through a function value carries no fact and the config value is never
// judged.
func ThroughValue(config SpinConfig, done func() bool) error {
	spin := lib.SpinReported
	return spin(done, config.Spins)
}

// BelowThreshold passes a constant that is large but under half of int.
//
// known-miss: a limit of 1 << 40 is a constant under half the type's
// maximum, so it passes even though no loop reaches it in practice.
func BelowThreshold(done func() bool) error {
	return lib.SpinReported(done, 1<<40)
}
