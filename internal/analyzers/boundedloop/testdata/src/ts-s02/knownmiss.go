// Documented TS-S02 coverage gaps: loops that read as syntactically bounded
// but whose body undoes the bound, so termination is not actually
// guaranteed. boundedloop only checks the Cond/Post syntax, never the body.
package fixture

// drainAppendKnownMiss is syntactically bounded by len(queue) > 0, but the
// body appends to queue on every round, so the loop can run forever.
//
// known-miss: boundedloop checks the shape of the condition, not whether the
// body keeps it shrinking; proving the queue actually drains needs the
// termination-chain analyzer from a later wave, so this stays silent.
func drainAppendKnownMiss(queue []int, feed func() (int, bool)) int {
	drained := 0
	for len(queue) > 0 {
		queue = queue[1:]
		drained++
		if next, ok := feed(); ok {
			queue = append(queue, next)
		}
	}
	return drained
}

// resetCounterKnownMiss is syntactically counter-bounded: the Post
// increments i and i appears in Cond. The body also resets i to 0 on some
// rounds, so the loop can run forever.
//
// known-miss: only the Post clause is checked against Cond; a body
// assignment that resets the counter is invisible to this syntactic check.
func resetCounterKnownMiss(limit int, again func() bool) int {
	rounds := 0
	for i := 0; i < limit; i++ {
		rounds++
		if again() {
			i = 0
		}
	}
	return rounds
}

// wrongDirectionLiteralKnownMiss is the pre-existing TS-S02 hole this wave
// deliberately does not close: x > 0 is accepted because 0 is a literal
// bound operand, with no check on which way x actually moves. x++ moves
// away from the exit, so the loop never terminates, but the grammar has
// no visibility into assignment direction for this shape — only the new
// != 0 widening gained that proof.
//
// known-miss: closing this needs a direction proof on x > 0 / x >= c the
// same way != 0 got one; deferred to hold the monotone-lenient constraint
// (this wave's widenings may only remove findings, never add one).
func wrongDirectionLiteralKnownMiss(x int) int {
	count := 0
	for x > 0 {
		x++
		count++
		if count > 1000000 {
			return count
		}
	}
	return count
}

// closureWrongDirectionKnownMiss's post right-shifts x toward zero, so
// the != 0 proof looks solid on its own — but a goroutine launched from
// the body also mutates x the wrong way.
//
// known-miss: FuncLit is a frame boundary for the direction scan (the
// same rule deferdistance and declusedistance apply), so the closure's
// assignment is invisible to the proof; seeing it needs a capture-escape
// analysis this single-package heuristic does not have.
func closureWrongDirectionKnownMiss(x uint, launch func(func())) int {
	count := 0
	for x != 0 {
		x >>= 1
		launch(func() {
			x++
		})
		count++
		if count > 1000000 {
			return count
		}
	}
	return count
}
