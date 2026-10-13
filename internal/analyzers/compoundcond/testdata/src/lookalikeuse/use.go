// Calls into a lookalike assert package earn no TS-S07 finding: only tiger's
// own import path is recognized as the assert package.
package fixture

import "lookalike/assert"

func checkPair(a, b bool) {
	assert.Ok(a && b, "a and b")
}
