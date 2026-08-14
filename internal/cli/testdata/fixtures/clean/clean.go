// Package clean is a conforming Tiger Go package: bounded loops, no naked
// panics, no directives.
package clean

// Total sums entries.
func Total(entries []int) int {
	total := 0
	for _, entry := range entries {
		total += entry
	}
	return total
}
