// The TS-N07 compliant rewrites: parameters of different types, an options
// struct instead of an adjacent same-type pair, and at most four
// parameters overall.
package fixture

// distinctTypes takes four parameters of four different types.
func distinctTypes(a int, b string, c float64, d byte) {
	_, _, _, _ = a, b, c, d
}

// copyOptions groups the formerly adjacent int pair into a named struct.
type copyOptions struct {
	Retries int
	Timeout int
}

// copyWithOptions replaces the adjacent-int pair with an options struct.
func copyWithOptions(source string, options copyOptions) {
	_, _ = source, options
}

// literalDistinct assigns a func literal with four different-typed
// parameters.
var literalDistinct = func(a int, b string, c float64, d byte) {
	_, _, _, _ = a, b, c, d
}

// CopierCompliant is an interface whose method takes an options struct
// instead of an adjacent same-type pair.
type CopierCompliant interface {
	// Copy replaces the adjacent-int pair with an options struct.
	Copy(source string, options copyOptions)
}
