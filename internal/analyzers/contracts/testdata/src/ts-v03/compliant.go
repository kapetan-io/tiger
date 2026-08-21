// The TS-V03 compliant shapes: constant arguments and returns that satisfy
// their pins, a variable argument guarded by a dominating nil check
// (unproven by constant reasoning, and also actually safe), and a call
// whose argument is an ordinary variable the analyzer cannot resolve to a
// compile-time value.
package fixture

// callNeedsPointerAddress passes the address of a local, satisfying
// needsPointer's requires p != nil.
func callNeedsPointerAddress() int {
	x := 5
	return needsPointer(&x)
}

// callNeedsPositiveFive passes a constant that satisfies n > 0.
func callNeedsPositiveFive() int {
	return needsPositive(5)
}

// callDecodeHeaderCorrectSize passes a 4-byte composite literal matching
// HeaderSizeBytes.
func callDecodeHeaderCorrectSize() int {
	return decodeHeader([]byte{1, 2, 3, 4})
}

// callGuardedPointer passes a variable argument to needsPointer, but only
// after a dominating nil check. The analyzer only reasons about literals,
// so it cannot see the check either way — this stays silent by
// construction, and the guard also happens to make the call genuinely
// safe.
func callGuardedPointer(p *int) int {
	if p == nil {
		return 0
	}
	return needsPointer(p)
}

// callNeedsPositiveVariable passes an ordinary variable; its value is not
// knowable at compile time, so the predicate stays unproven.
func callNeedsPositiveVariable(count int) int {
	return needsPositive(count)
}

// okNonNegative satisfies its own ensures with a correct constant return.
//
//tiger:ensures result >= 0
func okNonNegative() int {
	return 0
}
