// Counters that are sound although their type or limit looks risky: a
// generic integer, an unsigned count down to a constant, and a labeled cap
// that reports.
package fixture

import "errors"

// genericCount counts up to a limit of a generic integer type.
func genericCount[T ~int | ~int64](limit T) {
	for i := T(0); i < limit; i++ {
	}
}

// unsignedDownToOne stops at one, which a step of one reaches.
func unsignedDownToOne(n uint) {
	for i := n; i >= 1; i-- {
	}
}

// byteBelowParameter steps one at a time toward an exclusive limit.
func byteBelowParameter(n uint8) {
	for i := uint8(0); i < n; i++ {
	}
}

var errLabeledCap = errors.New("cap reached")

// labeledCapReports reports after a labeled loop.
func labeledCapReports(limit int, done func() bool) error {
	i := 0
labeled:
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errLabeledCap
	}
	_ = 0
	goto labeled
}

// byteToAssignedLimit counts a byte to a limit that is assigned a constant
// after its declaration.
func byteToAssignedLimit() {
	var limit uint8
	limit = 100
	for i := uint8(0); i < limit; i += 2 {
	}
}
