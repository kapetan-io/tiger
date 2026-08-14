// The TS-E06 compliant rewrites: zero, one, (T, error), and (T, bool)
// results.
package fixture

// noResult performs an action and returns nothing.
func noResult() {}

// oneResult returns a single value.
func oneResult() int {
	return 0
}

// withError returns a value and an error, the sanctioned two-result shape.
func withError() (int, error) {
	return 0, nil
}

// withBool returns a value and a bool, the other sanctioned two-result
// shape.
func withBool() (int, bool) {
	return 0, false
}

// literalWithError assigns a func literal in the compliant (T, error) shape.
var literalWithError = func() (string, error) {
	return "", nil
}

// AdderCompliant is an interface whose methods stay within the compliant
// shapes.
type AdderCompliant interface {
	// Add returns a value and an error.
	Add(a int) (int, error)
	// Halve returns a value and a bool.
	Halve(a int) (int, bool)
	// Reset returns nothing.
	Reset()
}
