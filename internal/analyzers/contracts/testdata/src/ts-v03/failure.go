// The TS-V03 proven-violation failure modes: a literal nil argument
// against requires p != nil, a compile-time-zero argument against requires
// n > 0, a wrong-length composite literal against a len() requires, and a
// constant-negative return against an ensures result >= 0.
package fixture

// HeaderSizeBytes is the fixed header length decodeHeader requires.
const HeaderSizeBytes = 4

// needsPointer requires a non-nil pointer; contracts proves a violation
// only when the argument is the literal nil.
//
//tiger:requires p != nil
func needsPointer(p *int) int {
	return *p
}

// callNeedsPointerNil passes the literal nil, provably false against
// needsPointer's precondition.
func callNeedsPointerNil() int {
	return needsPointer(nil) // want `TS-V03: this call violates //tiger:requires p != nil`
}

// needsPositive requires a positive count.
//
//tiger:requires n > 0
func needsPositive(n int) int {
	return n * 2
}

// callNeedsPositiveZero passes the compile-time constant 0, provably false
// against n > 0.
func callNeedsPositiveZero() int {
	return needsPositive(0) // want `TS-V03: this call violates //tiger:requires n > 0`
}

// decodeHeader requires target's length to match the fixed header size.
//
//tiger:requires len(target) == HeaderSizeBytes
func decodeHeader(target []byte) int {
	return len(target)
}

// callDecodeHeaderWrongSize passes a 3-element composite literal against a
// 4-byte requirement — both lengths are knowable at compile time.
func callDecodeHeaderWrongSize() int {
	return decodeHeader([]byte{1, 2, 3}) // want `TS-V03: this call violates //tiger:requires len\(target\) == HeaderSizeBytes`
}

// nonNegative ensures its result is never negative.
//
//tiger:ensures result >= 0
func nonNegative(n int) int {
	if n < 0 {
		return -1 // want `TS-V03: this return violates //tiger:ensures result >= 0`
	}
	return n
}
