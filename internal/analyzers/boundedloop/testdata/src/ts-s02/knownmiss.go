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
