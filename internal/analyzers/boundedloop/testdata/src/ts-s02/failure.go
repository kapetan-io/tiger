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
