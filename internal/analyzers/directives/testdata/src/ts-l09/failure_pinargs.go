// The TS-L09 blocking failure modes for pin arguments: a pin whose
// arguments fall outside its verb's grammar. The lattice and the predicate
// languages are closed, so a misspelled qualifier or an out-of-language
// expression is an error naming the token, never a silently meaningless pin.
package fixture

// MisspelledQualifier names an io qualifier outside the built-in tier.
//
// want +1 `TS-L09: //tiger:effects io qualifier "dsk" is not in the built-in tier`
//tiger:effects io(dsk)
func MisspelledQualifier() {}

// OutsideTheLattice names an effect the closed lattice does not contain.
//
// want +1 `TS-L09: //tiger:effects unknown effect "sleep"`
//tiger:effects sleep
func OutsideTheLattice() {}

// EmptyPin states no fact; purity is spelled none, not silence.
//
// want +1 `TS-L09: //tiger:effects an effects pin states a fact`
//tiger:effects
func EmptyPin() {}

// BrokenFrameLocation writes a location that is not a parameter-rooted path.
//
// want +1 `TS-L09: //tiger:frame frame location "r\.\.log" is not a parameter-rooted path`
//tiger:frame r..log
func BrokenFrameLocation() {}

// VariantOutsideTheLanguage writes a ranking the linear language cannot
// express; the loop needs the explicit counter-cap rewrite or a linear form.
//
// want +1 `TS-L09: //tiger:variant atom "n \* n" is outside the variant language`
//tiger:variant n * n
func VariantOutsideTheLanguage() {}

// ContractOutsideTheLanguage calls a function the predicate language does
// not admit; only len\(\) is a call.
//
// want +1 `TS-L09: //tiger:requires atom "checksum\(payload\)" is outside the predicate language`
//tiger:requires digest == checksum(payload)
func ContractOutsideTheLanguage(digest string, payload []byte) {}
