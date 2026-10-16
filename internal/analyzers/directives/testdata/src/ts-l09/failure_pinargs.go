// The TS-L09 blocking failure modes for pin arguments: a pin whose
// arguments fall outside its verb's grammar. The predicate languages are
// closed, so an out-of-language expression is an error naming the token,
// never a silently meaningless pin.
package fixture

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
