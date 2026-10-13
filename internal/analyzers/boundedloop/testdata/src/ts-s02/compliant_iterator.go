// A range over a standard library iterator is bounded by what it walks.
package fixture

import (
	"maps"
	"slices"
)

// sumValues ranges over a standard library iterator over a slice.
func sumValues(numbers []int) int {
	total := 0
	for number := range slices.Values(numbers) {
		total += number
	}
	return total
}

// countKeys ranges over a standard library iterator over a map.
func countKeys(entries map[string]int) int {
	total := 0
	for range maps.Keys(entries) {
		total++
	}
	return total
}
