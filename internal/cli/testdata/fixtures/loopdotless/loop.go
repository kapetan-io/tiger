package loopdotless

import "myapp/seq"

// Sum ranges over an iterator from this module, whose path has no dot.
func Sum() int {
	total := 0
	for value := range seq.Naturals() {
		total += value
	}
	return total
}
