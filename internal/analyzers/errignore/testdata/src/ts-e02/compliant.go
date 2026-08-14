// The compliant TS-E02 rewrites: every discarded error carries a comment,
// and discarding a non-error result or a range's blank binding stays
// silent regardless.
package fixture

import "os"

// closeCommented explains the discard in a trailing comment.
func closeCommented(f *os.File) {
	_ = f.Close() // best effort cleanup; the caller already checked write errors
}

// closeCommentedAbove explains the discard in a comment on the line above.
func closeCommentedAbove(f *os.File) {
	// best effort cleanup; nothing actionable if this fails
	_ = f.Close()
}

// lookupCommented explains why the error half is safe to discard.
func lookupCommented() int {
	v, _ := lookupTwo() // lookupTwo never returns a non-nil error today
	return v
}

func lookupTwo() (int, error) {
	return 0, nil
}

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
