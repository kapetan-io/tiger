// The TS-S02 limits that enter the program with no bound: a config value, a
// request field, a pack header, and a constant no loop reaches.
package fixture

import (
	"lib"
	"math"
	"store"
	"types"
	"wire"
)

// SpinConfig holds settings read from a file at startup.
type SpinConfig struct{ Spins int }

// ListRequest is a list request as decoded off the wire.
type ListRequest struct{ Limit int32 }

// SpinFromConfigRaw passes a config value straight to a loop limit.
func SpinFromConfigRaw(config SpinConfig, done func() bool) error {
	return lib.SpinReported(done, config.Spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// SpinForever passes the largest int, which no loop reaches.
func SpinForever(done func() bool) error {
	return lib.Spin(done, math.MaxInt) // want `TS-S02: this value is too large for the loop to ever reach`
}

// Forward passes its parameter on, so the question moves to its callers.
func Forward(done func() bool, n int) error { // want Forward:`\[1\]`
	return lib.SpinReported(done, n)
}

// ForwardCallers calls Forward with a bounded and an unbounded value.
func ForwardCallers(config SpinConfig, done func() bool) {
	_ = Forward(done, 50)
	_ = Forward(done, config.Spins) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// ListRaw writes the request's limit into the field a store loops to.
func ListRaw(lister store.Lister, rows store.Rows, req ListRequest) int {
	if req.Limit == 0 {
		req.Limit = 100
	}
	return lister.List(rows, types.ListOptions{Limit: int(req.Limit)}) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}

// ListAssigned assigns the request's limit into the field.
func ListAssigned(rows store.Rows, req ListRequest) int { // want ListAssigned:`\[\]`
	var opts types.ListOptions
	opts.Limit = int(req.Limit) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	return store.Memory{}.List(rows, opts)
}

// packCount reads the object count from a pack header.
func packCount(raw []byte) int {
	return int(wire.Uint32(raw[8:12]))
}

// parseObjects loops once per object the header claims.
func parseObjects(raw []byte, count int) int { // want parseObjects:`\[1\]` parseObjects:`\[1\]`
	parsed := 0
	for i := range count {
		parsed += i
	}
	return parsed + len(raw)
}

// IngestRaw hands the header's count straight to the loop.
func IngestRaw(raw []byte) int {
	count := int(wire.Uint32(raw[8:12])) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	return parseObjects(raw, count)
}

// CountRaw loops to a count it read from a function in this package.
func CountRaw(raw []byte) int { // want CountRaw:`\[\]`
	count := packCount(raw) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
	total := 0
	for i := 0; i < count; i++ {
		total += i
	}
	return total
}

// spread splits a request across backends.
func spread(requested int) []int {
	return []int{requested}
}

// CreateRaw loops once per requested partition, reached through spread.
func CreateRaw(requested int) int { // want CreateRaw:`\[\]`
	created := 0
	for _, count := range spread(requested) { // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
		for i := 0; i < count; i++ {
			created++
		}
	}
	return created
}

// SpinHuge caps a spin at a number no loop reaches.
func SpinHuge(done func() bool) {
	for i := 0; i < math.MaxInt && !done(); i++ { // want `TS-S02: this value is too large for the loop to ever reach`
	}
}

// validate rejects a limit above the maximum, but in another function.
func validate(req ListRequest) error {
	if req.Limit > 1000 {
		return lib.Spin(func() bool { return true }, 0)
	}
	return nil
}

// ListValidatedElsewhere is flagged even though validate rejects large
// values: the check is in a function that does not dominate the call, so
// tiger can't see it. A clamp the loop's own function makes is the form that
// passes.
func ListValidatedElsewhere(lister store.Lister, rows store.Rows, req ListRequest) int {
	if err := validate(req); err != nil {
		return 0
	}
	return lister.List(rows, types.ListOptions{Limit: int(req.Limit)}) // want `TS-S02: this value reaches a loop limit with no upper bound where it enters`
}
