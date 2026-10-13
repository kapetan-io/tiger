// Counter loops whose condition reads as a stated bound, a constant or a len,
// while the counter does not reach it: a count down that wraps, a step past
// the type's edge, and a limit the body grows. The counter is judged on its
// own, so TS-S02 blocks each of them, and so does TS-V01.
package fixture

// unsignedDown counts a uint down to and including zero, and zero minus one
// wraps back to the top.
func unsignedDown(n uint) {
	for i := n; i >= 0; i-- { // want `TS-S02: this counter does not bound the loop`
	}
}

// overflowStep steps a byte past its limit: 250 + 10 wraps to 4.
func overflowStep() {
	for i := uint8(0); i < 255; i += 10 { // want `TS-S02: this counter does not bound the loop`
	}
}

// growsMapLimit inserts into the map whose length is the limit.
func growsMapLimit(entries map[int]int) {
	for i := 0; i < len(entries); i++ { // want `TS-S02: this counter does not bound the loop`
		entries[i+len(entries)] = i
	}
}

// growsThroughPointer grows the slice behind a pointer, which is the limit.
func growsThroughPointer(p *[]int) {
	for i := 0; i < len(*p); i++ { // want `TS-S02: this counter does not bound the loop`
		*p = append(*p, 1)
	}
}

// growsAnElement grows the slice held in an element, which is the limit.
func growsAnElement(rows [][]int) {
	for i := 0; i < len(rows[0]); i++ { // want `TS-S02: this counter does not bound the loop`
		rows[0] = append(rows[0], 1)
	}
}
