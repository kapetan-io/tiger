// The TS-M05 compliant shapes: a reset immediately before Put, a reset
// present on every branch that reaches Put, and the zeroing-store form of a
// reset.
package fixture

// putAfterReset resets buf on the one straight-line path to Put.
func putAfterReset(buf *Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

// putAfterBothBranchesReset resets buf on every branch before the branches
// meet at the Put — must-reach through the meet is exactly why this
// analysis is per-path, not a single-dominator check.
func putAfterBothBranchesReset(cond bool, buf *Buffer) {
	if cond {
		buf.Reset()
	} else {
		buf.Reset()
	}
	bufferPool.Put(buf)
}

// putAfterZeroStore zeroes buf by storing a fresh zero-valued Buffer
// through it, the other canonical reset form besides a Reset() call.
func putAfterZeroStore(buf *Buffer) {
	*buf = Buffer{}
	bufferPool.Put(buf)
}
