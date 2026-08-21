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

// mergeUntilEitherDrains is bounded under && with only one bound-worthy
// operand: i < len(a) is a stated length bound, remaining() is an opaque
// call. && needs only one bounded arm, since the loop exits the instant i
// reaches len(a), no matter what remaining reports.
func mergeUntilEitherDrains(a []int, remaining func() bool) int {
	total := 0
	i := 0
	for i < len(a) && remaining() {
		total += a[i]
		i++
	}
	return total
}

// eitherBoundReached is bounded under || only because *both* operands are
// individually bounded — || requires every operand bounded, since either
// alone going false still lets the loop run on the other.
func eitherBoundReached(entries, extras []int) int {
	total := 0
	for i, k := 0, 0; i < len(entries) || k < len(extras); i, k = i+1, k+1 {
		if i < len(entries) {
			total += entries[i]
		}
		if k < len(extras) {
			total += extras[k]
		}
	}
	return total
}

// shiftToZero is bounded: x != 0 with a post that only right-shifts x is
// provably monotone toward the exit — each shift halves x, so it reaches
// zero in at most bits(x) iterations.
func shiftToZero(x uint) int {
	count := 0
	for x != 0 {
		x >>= 1
		count++
	}
	return count
}

// divideToZero is bounded the same way: division by a constant greater
// than one is monotone toward zero (division by one would never change x,
// so it does not qualify).
func divideToZero(x int) int {
	count := 0
	for x != 0 {
		x /= 3
		count++
	}
	return count
}

// postShiftWithContinue keeps its zero-moving shift in the Post clause,
// which Go runs on every iteration — continue included — so the skip
// cannot stall progress and the monotone proof holds.
func postShiftWithContinue(x uint, skip func() bool) int {
	count := 0
	for ; x != 0; x >>= 1 {
		if skip() {
			continue
		}
		count++
	}
	return count
}

// shiftBeforeContinue moves x as the first body statement, before any
// branch can skip it, so every iteration provably makes progress.
func shiftBeforeContinue(x uint, skip func() bool) int {
	count := 0
	for x != 0 {
		x >>= 1
		if skip() {
			continue
		}
		count++
	}
	return count
}

// shadowAfterShift declares an inner x after the counter has already
// moved: the shadow is a different object, so its wrong-direction
// increment neither helps nor defeats the outer proof.
func shadowAfterShift(x uint, big func() bool) int {
	count := 0
	for x != 0 {
		x >>= 1
		if big() {
			var x uint = 8
			x++
			_ = x
		}
		count++
	}
	return count
}

// closingGap is bounded by the tuple-post-reversal widening: the post
// moves i up and j down by the same step, so the gap shrinks every round
// and the < condition provably goes false.
func closingGap(entries []int) int {
	count := 0
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		count++
	}
	return count
}

// iterator is the opaque store-cursor shape boundedCond cannot see into:
// Valid() is a plain method call, not a comparison, so only the
// //tiger:batched cursor waiver can clear it.
type iterator struct{ n int }

func (it *iterator) Valid() bool { return it.n < 100 }
func (it *iterator) Next()       { it.n++ }

// rowsCursor is the other cursor shape the waiver covers: the boolean
// method call itself is the advance (the database-rows.Next() pattern).
type rowsCursor struct{ n int }

func (r *rowsCursor) Next() bool { r.n++; return r.n < 100 }

// cursorScanBatched is the cursor waiver's preceding-line directive
// placement: the directive sits in the comment group immediately above
// the loop it governs.
func cursorScanBatched(it *iterator) int {
	count := 0
	//tiger:batched store cursor is guaranteed finite by the backend
	for it.Valid() {
		it.Next()
		count++
	}
	return count
}

// cursorScanBatchedTrailing is the cursor waiver's trailing directive
// placement: the directive trails on the for line itself.
func cursorScanBatchedTrailing(rows *rowsCursor) int {
	count := 0
	for rows.Next() { //tiger:batched store cursor is guaranteed finite by the backend
		count++
	}
	return count
}
