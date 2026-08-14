// The TS-E06 failure modes: more than two results, and exactly two results
// whose last is neither error nor bool.
package fixture

// tripleResult returns the never-allowed (T, bool, error) shape.
func tripleResult() (int, bool, error) { // want `TS-E06: 3 results`
	return 0, false, nil
}

// threeResults returns three values of any composition; the count alone
// fires regardless of what the values are.
func threeResults() (int, int, error) { // want `TS-E06: 3 results`
	return 0, 0, nil
}

// pairOfSameType returns two values whose second is not error or bool.
func pairOfSameType() (int, int) { // want `TS-E06: second result`
	return 0, 0
}

// literalTriple assigns a func literal with the too-many-results failure.
var literalTriple = func() (int, bool, error) { // want `TS-E06: 3 results`
	return 0, false, nil
}

// literalPair assigns a func literal whose second result is neither error
// nor bool.
var literalPair = func() (string, string) { // want `TS-E06: second result`
	return "", ""
}

// Adder is an interface whose methods violate TS-E06.
type Adder interface {
	// Add returns three results.
	Add(a int) (int, bool, error) // want `TS-E06: 3 results`
	// Divide returns a pair whose second result is not error or bool.
	Divide(a int) (int, int) // want `TS-E06: second result`
}
