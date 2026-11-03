// A limit that adds to itself follows what it adds; a limit that feeds
// itself any other way never settles.
package fixture

import "lib"

// SpinAccumulatedRequest adds an unbounded request value to the limit.
func SpinAccumulatedRequest(request Tuning, done func() bool) error {
	limit := 1
	limit += request.Spins // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	return lib.SpinReported(done, limit)
}

// SpinDoubled starts from a constant but doubles on every pass.
func SpinDoubled(done func() bool) error {
	limit := 1
	for i := 0; i < 40; i++ {
		limit = limit * 2 // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	}
	return lib.SpinReported(done, limit)
}
