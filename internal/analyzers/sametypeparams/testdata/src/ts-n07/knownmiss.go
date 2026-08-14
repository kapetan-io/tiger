// known-miss: TS-N07 only flags adjacent same-type parameters, per the
// spec's own wording. A same-type pair separated by a different type, such
// as "a int, s string, b int", does not fire even though a and b could
// still be swapped by a caller working through the middle argument.
package fixture

// separatedSameType has two int parameters that are not adjacent, so the
// pair is a documented coverage gap rather than a bug.
func separatedSameType(a int, s string, b int) {
	_, _, _ = a, s, b
}

// literalCopy is a function literal with adjacent same-type parameters;
// literals are deliberately silent — their signatures are dictated by the
// consuming API (sort.Slice's func(i, j int) bool), so a finding on one is
// unfixable at the literal. A dogfood-surfaced false positive, fixed in the
// analyzer per correctness constraint 7.
var literalCopy = func(retries, timeout int) {
	_, _ = retries, timeout
}
