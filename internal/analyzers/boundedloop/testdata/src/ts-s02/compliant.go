// The TS-S02 compliant shapes: a bound visible in the syntax, either a
// constant, a len(), a shrinking length check, a range over a bounded
// container, or an explicit cap with an assert on exhaustion.
package fixture

import "assert"

const retriesMax = 5

const newtonItersMax = 20

// retryConstant counts up against a constant bound.
func retryConstant() int {
	count := 0
	for attempt := 0; attempt < retriesMax; attempt++ {
		count++
	}
	return count
}

// sumEntries counts up against a len() bound.
func sumEntries(entries []int) int {
	total := 0
	for i := 0; i < len(entries); i++ {
		total += entries[i]
	}
	return total
}

// drainPending shrinks pending on every iteration until it is empty.
func drainPending(pending []int) int {
	drained := 0
	for len(pending) > 0 {
		pending = pending[1:]
		drained++
	}
	return drained
}

// sumSlice ranges over a slice, which is bounded by its length.
func sumSlice(entries []int) int {
	total := 0
	for _, entry := range entries {
		total += entry
	}
	return total
}

// newtonCompliant is the notifier example's fix: the bound is now real,
// visible, and machine-proved, so a wrong convergence belief fails loudly at
// the cap instead of hanging forever.
func newtonCompliant(estimate float64) float64 {
	for i := 0; i < newtonItersMax; i++ {
		if converged(estimate) {
			break
		}
		estimate = refine(estimate)
	}
	assert.Ok(converged(estimate), "no convergence within newtonItersMax")
	return estimate
}
