// The TS-T02 compliant shapes: every body in the closed order-insensitive
// allowlist, plus the sorted-keys rewrite the finding always names.
package fixture

import (
	"maps"
	"math"
	"slices"
)

// copyMap duplicates src into a fresh map — each write only depends on its
// own key and value, never on visit order.
func copyMap(src map[string]int) map[string]int {
	dst := make(map[string]int)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// uniqueValues builds a set of m's values using struct{}{} sentinels — a
// write only depends on its own key, so it is order insensitive.
func uniqueValues(m map[string]int) map[int]struct{} {
	seen := make(map[int]struct{})
	for _, v := range m {
		seen[v] = struct{}{}
	}
	return seen
}

// markSeen builds a set of m's values using bool sentinels — the other
// common set-building idiom, equally order insensitive.
func markSeen(m map[string]int) map[int]bool {
	seen := make(map[int]bool)
	for _, v := range m {
		seen[v] = true
	}
	return seen
}

// sumValues totals every value in m — += is commutative, so the total does
// not depend on which order entries are visited.
func sumValues(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

// maxValue returns the largest value in m using the max builtin — order
// insensitive because max is commutative and associative.
func maxValue(m map[string]int) int {
	best := 0
	for _, v := range m {
		best = max(best, v)
	}
	return best
}

// minValueIfForm tracks the smallest value using the if-comparison shape,
// seeded from math.MaxInt so every element can only lower it — order
// insensitive because the final minimum does not depend on visit order.
func minValueIfForm(m map[string]int) int {
	best := math.MaxInt
	for _, v := range m {
		if v < best {
			best = v
		}
	}
	return best
}

// pruneZeros removes every zero-valued entry from m by ranging over it and
// calling delete — order insensitive regardless of visit order.
func pruneZeros(m map[string]int) {
	for k, v := range m {
		if v == 0 {
			delete(m, k)
		}
	}
}

// containsValue reports whether target appears among m's values, using a
// boolean short-circuit that breaks as soon as it finds one — the result
// does not depend on which order the map yields entries in.
func containsValue(m map[string]int, target int) bool {
	found := false
	for _, v := range m {
		if v == target {
			found = true
			break
		}
	}
	return found
}

// sortedValues is the rewrite every TS-T02 finding names: collect the keys,
// sort them, then range over the sorted slice instead of the map itself.
// The range target here is a slice, not a map, so maporder never inspects
// this loop's body at all.
func sortedValues(m map[string]int) []int {
	var result []int
	for _, k := range slices.Sorted(maps.Keys(m)) {
		result = append(result, m[k])
	}
	return result
}
