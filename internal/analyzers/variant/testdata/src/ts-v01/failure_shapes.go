// TS-V01 shrink forms that look like the learned ones and are not: a shift
// by nothing, a slice that keeps everything, a part that may be empty, and a
// subject that something else can reset.
package fixture

// shiftByZero shifts by a constant that makes no progress.
func shiftByZero(size uint64) int {
	count := 0
	for size > 0 { // want `TS-V01: tiger can't prove this loop ends`
		size >>= 0
		count++
	}
	return count
}

// divideAboveMinusOne lets zero through the condition.
func divideAboveMinusOne(value int) int {
	count := 0
	for value > -1 { // want `TS-V01: tiger can't prove this loop ends`
		value /= 2
		count++
	}
	return count
}

// divideByOther divides by something other than a constant.
func divideByOther(value, other int) int {
	count := 0
	for value > 0 { // want `TS-V01: tiger can't prove this loop ends`
		value = other / 2
		count++
	}
	return count
}

// sliceFromZero keeps the whole slice.
func sliceFromZero(data []int) {
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		data = data[0:]
	}
}

// trimNothing cuts nothing from the end.
func trimNothing(data []int) {
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		data = data[:len(data)-0]
	}
}

// emptyFront starts the part as an empty front.
func emptyFront(data []byte) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		chunk := data[:0]
		data = data[len(chunk):]
		count++
	}
	return count
}

// cutToEmpty cuts the part to nothing before slicing by it.
func cutToEmpty(data []byte) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		chunk := data
		chunk = chunk[:0]
		data = data[len(chunk):]
		count++
	}
	return count
}

// frontMayBeEmpty starts the part after the first byte, which is empty when
// one byte remains.
func frontMayBeEmpty(data []byte) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		chunk := data[1:]
		data = data[len(chunk):]
		count++
	}
	return count
}

// staleChunk slices by a part cut before the loop, which may be empty.
func staleChunk(data []byte, chunk []byte) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		data = data[len(chunk):]
		count++
	}
	return count
}

// chunkEscapes lets a pointer empty the part before the slice.
func chunkEscapes(data []byte) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		chunk := data
		clear := &chunk
		*clear = nil
		data = data[len(chunk):]
		count++
	}
	return count
}

// fieldChunk slices by a field that something else sets.
type fieldChunk struct{ chunk []byte }

func (f *fieldChunk) drain(data []byte) int {
	count := 0
	for len(data) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		data = data[len(f.chunk):]
		count++
	}
	return count
}

// rootSwapped shrinks a field of a struct the body swaps for another.
func rootSwapped(h, other *handler) {
	for len(h.items) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		h.items = h.items[1:]
		h = other
	}
}

// rootEscaped shrinks a field of a struct that a closure can swap.
func rootEscaped(h, other *handler) {
	swap := func() { h = other }
	for len(h.items) > 0 { // want `TS-V01: tiger can't prove this loop ends`
		h.items = h.items[1:]
		swap()
	}
}

// byteDownWraps steps a byte down by 5 while it is at least 3, and 4 - 5
// wraps to 255.
func byteDownWraps(size uint8) int {
	count := 0
	//tiger:variant size
	for size >= 3 { // want `TS-V01: //tiger:variant size doesn't provably shrink on every pass \(size can wrap around the end of its type before it stops\)`
		size -= 5
		count++
	}
	return count
}

// pinOnUnrelatedVariable pins n, which the condition i < m says nothing about.
func pinOnUnrelatedVariable(i, m, n int) {
	//tiger:variant n
	for i < m { // want `TS-V01: //tiger:variant n doesn't provably shrink on every pass \(the expression is about variables the loop's condition does not compare\)`
		n--
	}
}

// growsThroughPointer grows the slice behind a pointer, which is the limit.
func growsThroughPointer(p *[]int) {
	for i := 0; i < len(*p); i++ { // want `TS-V01: tiger can't prove this loop ends`
		*p = append(*p, 1)
	}
}

// growsAnElement grows the slice held in an element, which is the limit.
func growsAnElement(rows [][]int) {
	for i := 0; i < len(rows[0]); i++ { // want `TS-V01: tiger can't prove this loop ends`
		rows[0] = append(rows[0], 1)
	}
}

// byteToParameter counts a byte up to a parameter that may be 255.
func byteToParameter(n uint8) {
	for i := uint8(0); i <= n; i++ { // want `TS-V01: tiger can't prove this loop ends`
	}
}

// unsignedByTwo counts down by two toward a parameter and wraps past zero.
func unsignedByTwo(n, floor uint) {
	for floor < n { // want `TS-V01: tiger can't prove this loop ends`
		n -= 2
	}
}
