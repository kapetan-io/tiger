// The TS-S06 compliant rewrites: one logical operator per condition.
package fixture

// andChainCompliant chains a single operator: silent no matter how long the
// chain runs.
func andChainCompliant(a, b, c bool) bool {
	if a && b && c {
		return true
	}
	return false
}

// orChainCompliant chains a single operator: silent.
func orChainCompliant(a, b, c bool) bool {
	if a || b || c {
		return true
	}
	return false
}

// nestedCompliant replaces the mixed condition with nested if/else so each
// case is explicit and the || case has somewhere to live.
func nestedCompliant(a, b, c bool) bool {
	if a && b {
		return true
	}
	if c {
		return true
	}
	return false
}

// forSingleOperatorCompliant chains only && in the loop condition.
func forSingleOperatorCompliant(n int, done bool) int {
	total := 0
	for n > 0 && !done {
		n--
		total++
	}
	return total
}
