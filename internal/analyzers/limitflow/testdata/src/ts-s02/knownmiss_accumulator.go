// A limit built by v += x is judged by x alone.
package fixture

import "lib"

// SpinAccumulatedConstant adds a constant on every pass, so the limit grows
// with the loop count.
//
// known-miss: v += x is trusted to be as bounded as x, since an accumulator
// runs in loops that are themselves bounded; the growth is not multiplied
// out.
func SpinAccumulatedConstant(done func() bool) error {
	limit := 1
	for i := 0; i < 40; i++ {
		limit += 1000
	}
	return lib.SpinReported(done, limit)
}
