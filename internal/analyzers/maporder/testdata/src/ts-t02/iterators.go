// Item 3: every maps.Keys, maps.Values and maps.All result must reach a
// sort that fixes its order, a map, or a range (judged as a map range).
package fixture

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sort"
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

// rankByValue sorts keys by value, so keys with equal values tie.
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

// unsortedCollect collects m's keys via slices.Collect(maps.Keys(m))
// without sorting them, then ranges over the resulting slice. The iterator
// check fires on the Collect call.
func unsortedCollect(m map[string]int) []int {
	var result []int
	for _, k := range slices.Collect(maps.Keys(m)) { // want `TS-T02: this iterator yields map entries`
		result = append(result, m[k])
	}
	return result
}

// Collected into a local slice that a plain sort orders next.
func collectThenSortStrings(m map[string]int) []string {
	keys := slices.Collect(maps.Keys(m))
	sort.Strings(keys)
	return keys
}

func appendSeqThenSort(m map[string]int, prefix []string) []string {
	prefix = slices.AppendSeq(prefix, maps.Keys(m))
	slices.Sort(prefix)
	return prefix
}

// The slice is read before the sort.
func collectThenRead(m map[string]int) []string {
	keys := slices.Collect(maps.Keys(m)) // want `TS-T02: this iterator`
	first := keys[0]
	sort.Strings(keys)
	return append(keys, first)
}

// Floats can't be told apart by a sort.
func collectFloatsThenSort(m map[string]float64) []float64 {
	vals := slices.Collect(maps.Values(m)) // want `TS-T02: this iterator`
	slices.Sort(vals)
	return vals
}

// Printed through a closure before the sort.
func collectThenClosure(m map[string]int) {
	var keys []string
	show := func() { fmt.Println(keys) }
	keys = slices.Collect(maps.Keys(m)) // want `TS-T02: this iterator`
	show()
	sort.Strings(keys)
}

func appendSeqFresh(m map[string]int) []string {
	keys := slices.AppendSeq(make([]string, 0, len(m)), maps.Keys(m))
	slices.Sort(keys)
	return keys
}
