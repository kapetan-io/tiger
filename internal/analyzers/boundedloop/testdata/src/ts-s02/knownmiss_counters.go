// Counter loops TS-S02 accepts because the condition compares against a
// constant or a len, which the rule takes as a stated bound. TS-V01 blocks each
// of them, so tiger check still fails the build.
package fixture

// known-miss: the loops below read as bounded by their constant or len
// operand; whether the counter reaches it is TS-V01's question, not TS-S02's.

// unsignedDown counts a uint down to and including zero, and zero minus one
// wraps back to the top.
func unsignedDown(n uint) {
	for i := n; i >= 0; i-- {
	}
}

// overflowStep steps a byte past its limit: 250 + 10 wraps to 4.
func overflowStep() {
	for i := uint8(0); i < 255; i += 10 {
	}
}

// growsMapLimit inserts into the map whose length is the limit.
func growsMapLimit(entries map[int]int) {
	for i := 0; i < len(entries); i++ {
		entries[i+len(entries)] = i
	}
}

// growsThroughPointer grows the slice behind a pointer, which is the limit.
func growsThroughPointer(p *[]int) {
	for i := 0; i < len(*p); i++ {
		*p = append(*p, 1)
	}
}

// growsAnElement grows the slice held in an element, which is the limit.
func growsAnElement(rows [][]int) {
	for i := 0; i < len(rows[0]); i++ {
		rows[0] = append(rows[0], 1)
	}
}
