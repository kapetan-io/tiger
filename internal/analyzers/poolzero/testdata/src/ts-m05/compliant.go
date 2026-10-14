// The TS-M05 compliant shapes: *b = T{} immediately before Put, on every
// branch that reaches Put, and right after the value came out of the pool.
package fixture

// putAfterZero zeroes buf on the one straight-line path to Put.
func putAfterZero(buf *Buffer) {
	*buf = Buffer{}
	bufferPool.Put(buf)
}

// putAfterBothBranchesZero zeroes buf on every branch before the branches
// meet at the Put — must-reach through the meet is exactly why this
// analysis is per-path, not a single-dominator check.
func putAfterBothBranchesZero(cond bool, buf *Buffer) {
	if cond {
		*buf = Buffer{}
	} else {
		*buf = Buffer{}
	}
	bufferPool.Put(buf)
}

// getUseZeroAndPutBack zeroes a buffer it took from the pool before putting
// it back. The zeroing store sits on a value that came straight from Get,
// the shape call 7 found the analyzer mistook for an unreset Put.
func getUseZeroAndPutBack(entry byte) {
	buf, ok := bufferPool.Get().(*Buffer)
	if !ok {
		return
	}
	buf.data = append(buf.data, entry)
	*buf = Buffer{}
	bufferPool.Put(buf)
}

// getUseZeroSingleValue is the same shape with the single-result type
// assertion, which compiles to a different SSA form.
func getUseZeroSingleValue(entry byte) {
	buf := bufferPool.Get().(*Buffer)
	buf.data = append(buf.data, entry)
	*buf = Buffer{}
	bufferPool.Put(buf)
}
