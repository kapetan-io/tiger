// The TS-S02 sound counters: each compares the counter with a limit nothing
// changes, in a top-level && term, and steps toward it.
package fixture

import "go/types"

// countUp counts up toward a parameter limit.
func countUp(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}

// countDown counts down with >= toward a constant.
func countDown(n int) int {
	total := 0
	for i := n; i >= 1; i-- {
		total += i
	}
	return total
}

// countStep steps by 2 toward its limit.
func countStep(n int) int {
	total := 0
	for i := 0; i < n; i += 2 {
		total += i
	}
	return total
}

// limitOnTheLeft writes the comparison with the limit first.
func limitOnTheLeft(n int) int {
	total := 0
	for i := 0; n > i; i++ {
		total += i
	}
	return total
}

// countToByteMax stops one short of the byte's maximum, so the counter never
// wraps.
func countToByteMax() int {
	total := 0
	for i := uint8(0); i < 255; i++ {
		total++
	}
	return total
}

// countFields walks a tuple by the count it reports, a declared method the
// body does not touch.
func countFields(tuple *types.Tuple) int {
	total := 0
	for i := 0; i < tuple.Len(); i++ {
		total += len(tuple.At(i).Name())
	}
	return total
}

// register holds a limit in a field.
type register struct{ limit int }

// countField counts toward a field nothing writes.
func countField(r register) int {
	total := 0
	for i := 0; i < r.limit; i++ {
		total++
	}
	return total
}

// countUntilFound counts toward len(items) and leaves early; len is a bound
// the data sets, so the early exit needs no report.
func countUntilFound(items []int, want int) int {
	for i := 0; i < len(items) && items[i] != want; i++ {
	}
	return want
}
