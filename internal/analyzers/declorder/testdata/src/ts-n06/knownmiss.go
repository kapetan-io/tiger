// known-miss: TS-N06 only fires when a helper's entire call set is certain
// — every identifier reference to it must sit in call position. buildQuery
// is called once, by fetchResults, which would make it a single-caller
// helper with a mismatched name; but it is also assigned to queryBuilder as
// a bare value, so the analyzer cannot trust the one visible call site as
// the helper's true caller count and stays silent rather than guess.
package fixture

func fetchResults() []byte {
	return buildQuery()
}

var queryBuilder = buildQuery

func buildQuery() []byte {
	return nil
}
