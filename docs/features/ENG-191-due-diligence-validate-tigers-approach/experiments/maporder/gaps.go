package maporder

import (
	"maps"
	"slices"
	"sort"
)

// FirstThree copies whichever three entries the runtime visits first. Each
// statement is an allowlisted shape, but the counter carried across
// iterations decides which entries get written.
func FirstThree(m map[string]int) map[string]int {
	out := map[string]int{}
	n := 0
	for k, v := range m {
		n++
		if n <= 3 {
			out[k] = v
		}
	}
	return out
}

// RankByValue sorts keys by their values, and keys with equal values keep the
// map's order. The range target is an iterator, not a map, so TS-T02 never
// looks at it.
func RankByValue(m map[string]int) []string {
	return slices.SortedFunc(maps.Keys(m), func(a, b string) int { return m[a] - m[b] })
}

// SortedValues sorts float values. -0 and +0 compare equal, so their printed
// order follows the map's order.
func SortedValues(m map[string]float64) []float64 {
	values := make([]float64, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	sort.Float64s(values)
	return values
}
