// Item 3: every maps.Keys, maps.Values and maps.All result must reach a
// sort that fixes its order, a map, or a range (judged as a map range).
package fixture

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

func consume(seq iter.Seq[string]) {}

func collectKeysUnsorted(m map[string]int) []string {
	return slices.Collect(maps.Keys(m)) // want `TS-T02: this iterator`
}

func iteratorStored(m map[string]int) []string {
	keys := maps.Keys(m) // want `TS-T02: this iterator`
	return slices.Sorted(keys)
}

func iteratorPassed(m map[string]int) {
	consume(maps.Keys(m)) // want `TS-T02: this iterator`
}

func appendSeq(dst []int, m map[string]int) []int {
	return slices.AppendSeq(dst, maps.Values(m)) // want `TS-T02: this iterator`
}

// slices.Sorted on float values: -0 and +0.
func sortedFloatValues(m map[string]float64) []float64 {
	return slices.Sorted(maps.Values(m)) // want `TS-T02: this iterator`
}

// experiments/maporder's RankByValue: keys with equal values tie.
func rankByValue(m map[string]int) []string {
	return slices.SortedFunc(maps.Keys(m), func(a, b string) int { return m[a] - m[b] }) // want `TS-T02: this iterator`
}

func rankByValueCmp(m map[string]int) []string {
	return slices.SortedStableFunc(maps.Keys(m), func(a, b string) int { return cmp.Compare(m[a], m[b]) }) // want `TS-T02: this iterator`
}

func rangeIteratorAppend(m map[string]int) []string {
	var names []string
	for name := range maps.Keys(m) { // want `TS-T02: this loop appends`
		names = append(names, name)
	}
	return names
}

// --- accepted ---

func sortedIntValues(m map[string]int) []int { return slices.Sorted(maps.Values(m)) }

func rankByValueThenKey(m map[string]int) []string {
	return slices.SortedFunc(maps.Keys(m), func(a, b string) int {
		return cmp.Or(cmp.Compare(m[a], m[b]), cmp.Compare(a, b))
	})
}

func insertAll(dst, src map[string]int) { maps.Insert(dst, maps.All(src)) }

func rangeAllCopy(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range maps.All(m) {
		out[k] = v
	}
	return out
}

func rangeKeysCollectSorted(m map[string]int) []string {
	var names []string
	for name := range maps.Keys(m) {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
