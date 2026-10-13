// A test's table values are not input from outside, so a limit that comes
// straight from a table is not traced in a _test.go file.
package fixture

import "lib"

// spinFromTable passes a raw table value to a loop limit.
func spinFromTable(config SpinConfig, done func() bool) error {
	return lib.SpinReported(done, config.Spins)
}
