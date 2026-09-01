// The TS-L09 blocking failure modes for the restrict declaration: the one
// intent verb an analyzer reads, so its axes are a closed set like a pin's
// arguments — an unknown axis, an empty imports(), or unbalanced
// parentheses is an error naming the token, never a silently meaningless
// claim.
package fixture

// UnknownAxis names an axis the grammar does not have.
//
// want +1 `TS-L09: //tiger:restrict names "closed-world", which is not a restriction tiger knows`
//tiger:restrict closed-world
func UnknownAxis() {}

// EmptyImports claims an import list with nothing in it.
//
// want +1 `TS-L09: //tiger:restrict has an empty imports\(\)`
//tiger:restrict imports()
func EmptyImports() {}

// Unclosed leaves a parenthesis open.
//
// want +1 `TS-L09: //tiger:restrict has an unclosed "\("`
//tiger:restrict imports(internal/domain
func Unclosed() {}
