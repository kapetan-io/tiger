// More TS-S02 sound counters: unsigned counts down, steps down by a
// constant, and a comparison written with the counter on the right.
package fixture

// countDownUnsigned stops before zero, so the counter never wraps.
func countDownUnsigned(n uint) int {
	total := 0
	for i := n; i > 0; i-- {
		total++
	}
	return total
}

// countDownStep steps down by 2 toward a variable limit.
func countDownStep(n, floor int) int {
	total := 0
	for i := n; i > floor; i -= 2 {
		total++
	}
	return total
}

// limitOnTheLeftDown writes the downward comparison with the limit first.
func limitOnTheLeftDown(n, floor int) int {
	total := 0
	for i := n; floor < i; i-- {
		total++
	}
	return total
}

// breakInSwitch only leaves the switch, so the counter is the loop's one way
// out.
func breakInSwitch(limit int, kind func() int) int {
	total := 0
	for i := 0; i < limit; i++ {
		switch kind() {
		case 1:
			break
		default:
			total++
		}
	}
	return total
}

// breakInInnerLoop only leaves the inner loop.
func breakInInnerLoop(limit int, done func() bool) int {
	total := 0
	for i := 0; i < limit; i++ {
		for !done() && total < 3 {
			break
		}
		total++
	}
	return total
}
