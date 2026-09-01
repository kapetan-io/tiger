// Package closuresite passes AssertedOnce to assert.Invariant a second
// time from a different package, proving one production site already
// satisfies TS-A07 without requiring a second.
package closuresite

import (
	"fixture.example/invariantrefs/assert"
	"fixture.example/invariantrefs/inv"
)

// Recheck is a second call site for AssertedOnce's ID type, exercising
// NeverReferenced's type recognition: passing AssertedOnce here (not
// NeverReferenced) means NeverReferenced's type is recognized as an
// invariant type through AssertedOnce and NeverReferenced itself stays at
// zero references.
func Recheck(ok bool) {
	assert.Invariant(inv.AssertedOnce, ok)
}
