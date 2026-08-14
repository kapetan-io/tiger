// known-miss: TS-S06 only examines the syntactic Cond expression of an
// IfStmt or ForStmt. A mixed &&/|| expression computed earlier and carried
// through a variable is one level removed from that Cond, so the analyzer
// never re-derives it and the branch stays silent even though the same
// truth table applies.
package fixture

// mixedThroughVariable assigns a mixed &&/|| expression to a variable, then
// branches on the variable instead of the expression itself.
func mixedThroughVariable(a, b, c bool) bool {
	mixed := a && b || c
	if mixed {
		return true
	}
	return false
}
