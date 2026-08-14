// known-miss: participle's declared set is types, funcs, methods, consts,
// vars, and struct fields — interface methods are not walked, so an
// exported interface method ending in a participle stays silent even
// though it is exactly the trailing-participle shape TS-N14 exists to
// catch.
package fixture

// Runner has an exported interface method ending in a participle.
type Runner interface {
	Marshaling()
}
