// The TS-S07 failure modes: assert.Ok and assert.Invariant conditions that
// join checks with &&.
package fixture

import "assert"

// checkPairOk joins two checks with && inside assert.Ok's condition.
func checkPairOk(a, b bool) {
	assert.Ok(a && b, "a and b") // want `TS-S07: assert\.Ok condition combines checks with &&`
}

// checkTripleOk joins three checks with &&.
func checkTripleOk(a, b, c bool) {
	assert.Ok(a && b && c, "all three") // want `TS-S07: assert\.Ok condition combines checks with &&`
}

// checkInvariant joins two checks with && inside assert.Invariant's second
// argument.
func checkInvariant(id string, a, b bool) {
	assert.Invariant(id, a && b) // want `TS-S07: assert\.Invariant condition combines checks with &&`
}
