// The compliant TS-E02 shapes: discarding a non-error result or a range's
// blank binding stays silent. Handling the error is the only way to clear a
// discarded one, so no discard of an error appears here.
package fixture

// nonErrorDiscard shows a discarded non-error result never fires.
func nonErrorDiscard() int {
	total, _ := sumTwo(), 0
	return total
}

func sumTwo() int {
	return 1
}

// rangeDiscard shows a range statement's blank binding is not an
// AssignStmt this rule inspects.
func rangeDiscard(rows []int) int {
	total := 0
	for _, v := range rows {
		total += v
	}
	return total
}
