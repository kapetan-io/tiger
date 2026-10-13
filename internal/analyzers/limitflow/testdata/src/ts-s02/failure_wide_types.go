// A constant in the upper half of a 32-bit type is out of reach.
package fixture

// spinWide loops to a 32-bit limit.
func spinWide(done func() bool, limit int32) { // want spinWide:`\[1\]`
	for i := int32(0); i < limit && !done(); i++ {
	}
	assertDone(done)
}

// SpinWideHuge passes the largest 32-bit number.
func SpinWideHuge(done func() bool) {
	spinWide(done, 1<<31-1) // want `TS-S02: this value is too large for the loop to ever reach`
}
