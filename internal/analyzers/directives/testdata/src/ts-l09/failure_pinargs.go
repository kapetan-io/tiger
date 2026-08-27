// The TS-L09 blocking failure modes for pin arguments: a pin whose
// arguments fall outside its verb's grammar. The lattice and the predicate
// languages are closed, so a misspelled qualifier or an out-of-language
// expression is an error naming the token, never a silently meaningless pin.
package fixture

// MisspelledQualifier names an io qualifier outside the built-in tier.
//
// want +1 `TS-L09: //tiger:effects has io qualifier "dsk", which tiger doesn't know`
//tiger:effects io(dsk)
func MisspelledQualifier() {}

// OutsideTheLattice names an effect the closed lattice does not contain.
//
// want +1 `TS-L09: //tiger:effects names "sleep", which is not an effect tiger tracks`
//tiger:effects sleep
func OutsideTheLattice() {}

// EmptyPin states no fact; purity is spelled none, not silence.
//
// want +1 `TS-L09: //tiger:effects has nothing after it`
//tiger:effects
func EmptyPin() {}

// BrokenFrameLocation writes a location that is not a parameter-rooted path.
//
// want +1 `TS-L09: //tiger:frame has location "r\.\.log", which is not a path starting at a parameter or receiver`
//tiger:frame r..log
func BrokenFrameLocation() {}

// VariantOutsideTheLanguage writes a ranking the linear language cannot
// express; the loop needs the explicit counter-cap rewrite or a linear form.
//
// want +1 `TS-L09: //tiger:variant has "n \* n", which is not an integer, a path, or len\(path\)`
//tiger:variant n * n
func VariantOutsideTheLanguage() {}

// ContractOutsideTheLanguage calls a function the predicate language does
// not admit; only len\(\) is a call.
//
// want +1 `TS-L09: //tiger:requires has "checksum\(payload\)", which is not an integer, nil, a path, or len\(path\)`
//tiger:requires digest == checksum(payload)
func ContractOutsideTheLanguage(digest string, payload []byte) {}
