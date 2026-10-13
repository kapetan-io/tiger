// Constants in the upper half of a type narrower than 32 bits are not out of
// reach, because a loop can count to the edge of such a type.
package fixture

// spinNarrow loops to a 16-bit limit.
func spinNarrow(done func() bool, limit uint16) { // want spinNarrow:`\[1\]`
	for i := uint16(0); i < limit && !done(); i++ {
	}
	assertDone(done)
}

func assertDone(done func() bool) {
	if !done() {
		panic("not done")
	}
}

// SpinNarrowHuge passes the largest 16-bit number, which a loop reaches.
func SpinNarrowHuge(done func() bool) {
	spinNarrow(done, 65535)
}
