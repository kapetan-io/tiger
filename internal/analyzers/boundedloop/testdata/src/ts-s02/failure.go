// The TS-S02 failure modes: a for{} with no select at all, a condition that
// is not derived from a constant, a len, or a counter, and a range over a
// channel.
package fixture

// converged reports whether estimate is close enough to stop; the tool
// cannot see that the call itself converges.
func converged(estimate float64) bool {
	return estimate < 0.0001
}

// refine narrows the estimate for the next iteration.
func refine(estimate float64) float64 {
	return estimate / 2
}

// pollForever has no condition and no ctx.Done() select, so nothing in its
// syntax bounds how long it can run.
func pollForever() int {
	count := 0
	for { // want `TS-S02: this loop has no condition and no ctx\.Done\(\) select`
		count++
		if count > 1000000 {
			return count
		}
	}
}

// newton iterates until converged reports true, with no explicit cap — the
// termination argument lives in the reader's head, not in the syntax.
func newton(estimate float64) float64 {
	for !converged(estimate) { // want `TS-S02: this loop's bound cannot be derived`
		estimate = refine(estimate)
	}
	return estimate
}

// drain ranges over a channel, whose termination depends on another
// goroutine closing it.
func drain(work chan int) int {
	total := 0
	for value := range work { // want `TS-S02: ranging over a channel`
		total += value
	}
	return total
}

// pollActive loops on a plain bool variable with no counter and no constant
// bound.
func pollActive() int {
	active := true
	count := 0
	for active { // want `TS-S02: this loop's bound cannot be derived`
		count++
		if count > 3 {
			active = false
		}
	}
	return count
}

// orOneUnbounded's condition is a disjunction where only one arm is
// bounded; || requires every arm bounded, so the unbounded active arm
// still lets this loop run without a derivable cap.
func orOneUnbounded(limit int) int {
	active := true
	count := 0
	for active || count < limit { // want `TS-S02: this loop's bound cannot be derived`
		count++
		if count > 1000 {
			active = false
		}
	}
	return count
}

// neqZeroBareDecrement's condition looks monotone but a bare decrement
// under != 0 does not qualify: a negative start would wrap around instead
// of hitting zero, so no proof admits it.
func neqZeroBareDecrement(x int) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		x--
		count++
	}
	return count
}

// neqZeroWrongDirection has a qualifying post (a right shift) but the body
// also increments x — an assignment moving the wrong way defeats the
// proof no matter what else in the loop is provably monotone.
func neqZeroWrongDirection(x int) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		x >>= 1
		x++
		count++
	}
	return count
}

// cursorScanUnannotated is cursor-shaped but carries no //tiger:batched
// directive, so the waiver never applies and the loop still fires.
func cursorScanUnannotated(it *iterator) int {
	count := 0
	for it.Valid() { // want `TS-S02: this loop's bound cannot be derived`
		it.Next()
		count++
	}
	return count
}

// nonCursorBatchedStillFires is not cursor-shaped — its condition is a
// plain identifier, not a boolean method call — so an attached batched
// directive does not waive it; the waiver is shape-gated, not a blanket
// suppression.
func nonCursorBatchedStillFires(active bool) int {
	count := 0
	//tiger:batched not actually a cursor, this annotation should not help
	for active { // want `TS-S02: this loop's bound cannot be derived`
		count++
	}
	return count
}

// paginationTokenFires is the loop-carried-token pagination shape: even
// annotated, it gets no TS-S02 waiver, because the cursor waiver only
// recognizes a boolean method call on a plain identifier as the cursor
// shape, and a loop-carried token condition is not that shape. This is
// why the limitation is a corpus failure case, not a silent known-miss.
//
//tiger:batched next-page token pagination is bounded by the backend
func paginationTokenFires(fetch func(token string) ([]int, string, bool)) int {
	token, more, total := "", true, 0
	for more { // want `TS-S02: this loop's bound cannot be derived`
		var items []int
		items, token, more = fetch(token)
		total += len(items)
	}
	return total
}

// tupleMirrorFires is the >/>= mirror of the tuple-post-reversal shape,
// out of scope for the widening: only </<= gets a shrinking-gap proof, so
// the reversed comparison still fires — a documented limitation recorded
// here rather than as a silent known-miss (silence is reserved for shapes
// the grammar cannot see at all; this one is visible, just not admitted).
func tupleMirrorFires(i, j int) int {
	count := 0
	for i > j { // want `TS-S02: this loop's bound cannot be derived`
		i, j = i-1, j+1
		count++
	}
	return count
}

// foreverBatchedStillFires is a condition-less for{} carrying a batched
// annotation: the waiver is gated on the cursor shape, and an event loop
// is not a cursor, so the directive cannot become a blanket infinite-loop
// suppression.
func foreverBatchedStillFires() int {
	count := 0
	//tiger:batched no shape at all; this annotation should not help
	for { // want `TS-S02: this loop has no condition and no ctx\.Done\(\) select`
		count++
		if count > 10 {
			return count
		}
	}
}

// negatedConjunctionFires: !(done && i < limit) is !done || i >= limit by
// De Morgan, so the bounded arm alone proves nothing — while done stays
// false the loop runs forever regardless of i. Negation must flip the
// &&-either rule to the ||-every rule.
func negatedConjunctionFires(done bool, i int) int {
	count := 0
	for !(done && i < 10) { // want `TS-S02: this loop's bound cannot be derived`
		count++
		i++
	}
	return count
}

// signedShiftFires: an arithmetic right shift on a signed negative value
// converges to -1, never zero, so x != 0 with a signed shift has no
// termination proof — only an unsigned counter qualifies.
func signedShiftFires(x int) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		x >>= 1
		count++
	}
	return count
}

// variableShiftFires: a shift by a variable amount proves nothing — the
// amount can be zero, and x >> 0 never moves.
func variableShiftFires(x uint, k uint) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		x >>= k
		count++
	}
	return count
}

// floatDivideFires: floating division has values division never moves —
// NaN/2 is NaN and Inf/2 is Inf — so the monotone-to-zero proof holds for
// integer counters only.
func floatDivideFires(f float64) int {
	count := 0
	for f != 0 { // want `TS-S02: this loop's bound cannot be derived`
		f /= 2
		count++
	}
	return count
}

// rangeResetFires: a range clause assigning the counter (for x = range)
// is an assignment the direction proof must see — the inner range resets
// x after every division, so the outer loop never has to reach zero.
func rangeResetFires(x int, items []int) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		x /= 2
		for x = range items {
			count++
		}
	}
	return count
}

// shadowedShiftFires: the qualifying shift targets a shadowed inner x, not
// the loop counter — the outer x is never assigned, so the loop can spin
// forever. The direction proof must match assignments by object identity,
// never by name.
func shadowedShiftFires(x uint, big func() bool) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		if big() {
			var x uint = 8
			x >>= 1
			_ = x
		}
		count++
		if count > 1000000 {
			return count
		}
	}
	return count
}

// conditionalShiftFires: the only zero-moving assignment sits under an if
// whose guard may never be true, so no iteration is required to move x —
// the proof needs an assignment that provably runs every round.
func conditionalShiftFires(x uint, sometimes func() bool) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		if sometimes() {
			x >>= 1
		}
		count++
		if count > 1000000 {
			return count
		}
	}
	return count
}

// continueSkipsShiftFires: the shift is a direct body statement, but a
// continue before it re-enters the condition without moving x — with no
// Post clause, nothing forces progress on the skipped rounds.
func continueSkipsShiftFires(x uint, skip func() bool) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		if skip() {
			continue
		}
		x >>= 1
		count++
	}
	return count
}

// branchedShiftFires: both arms of the if move x toward zero, so the loop
// does terminate — but the grammar's unconditional-progress rule only
// accepts a direct body statement or the Post clause, so this fires. A
// documented conservative limitation, not a silent known-miss: restate the
// move unconditionally (or hoist it to Post) to pass.
func branchedShiftFires(x uint, pick func() bool) int {
	count := 0
	for x != 0 { // want `TS-S02: this loop's bound cannot be derived`
		if pick() {
			x >>= 1
		} else {
			x /= 2
		}
		count++
	}
	return count
}

// floatTupleFires: the tuple-post shrinking-gap proof holds for integers
// only — at large float magnitudes a+1 is absorbed (a+1 == a), so the gap
// stops shrinking and the loop never exits.
func floatTupleFires(lo, hi float64) int {
	count := 0
	for ; lo < hi; lo, hi = lo+1, hi-1 { // want `TS-S02: this loop's bound cannot be derived`
		count++
	}
	return count
}
