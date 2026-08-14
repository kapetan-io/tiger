// The TS-S06 failure modes: conditions that mix && and ||.
package fixture

// mixedIf mixes && and || with no parentheses.
func mixedIf(a, b, c bool) bool {
	if a && b || c { // want `TS-S06: condition mixes && and \|\|`
		return true
	}
	return false
}

// mixedParenIf mixes && and || with the && group parenthesized.
func mixedParenIf(a, b, c bool) bool {
	if (a && b) || c { // want `TS-S06: condition mixes && and \|\|`
		return true
	}
	return false
}

// mixedFor mixes && and || in a for loop's condition.
func mixedFor(n int, done bool) int {
	total := 0
	for n > 0 && !done || total < 10 { // want `TS-S06: condition mixes && and \|\|`
		n--
		total++
	}
	return total
}

// mixedNegated mixes && and || through a negated parenthesized group.
func mixedNegated(a, b, c bool) bool {
	if !(a && b) || c { // want `TS-S06: condition mixes && and \|\|`
		return true
	}
	return false
}
