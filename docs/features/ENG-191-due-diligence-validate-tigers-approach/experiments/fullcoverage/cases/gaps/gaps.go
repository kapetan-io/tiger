// Package gaps holds the five functions of experiments/maporder: the three
// gaps call 8's deliberation found in tiger's TS-T02 (FirstThree,
// RankByValue, SortedValues) and querator's collect-then-sort shape
// (SortedIDs, from querator@1fd1bb2 internal/store/memory.go:1054) with its
// unsorted twin.
package gaps

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"

	"fullcoverage/cases/variant"
)

// FirstThree copies whichever three entries the runtime visits first.
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

// RankByValue sorts keys by their values; keys with equal values keep the
// map's order.
func RankByValue(m map[string]int) []string {
	return slices.SortedFunc(maps.Keys(m), func(a, b string) int { return m[a] - m[b] })
}

// SortedValues sorts float values; -0 and +0 compare equal.
func SortedValues(m map[string]float64) []float64 {
	values := make([]float64, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	sort.Float64s(values)
	return values
}

// SortedIDs collects then sorts with sort.Strings.
func SortedIDs(m map[string]int) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// UnsortedIDs is SortedIDs without the sort.
func UnsortedIDs(m map[string]int) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

func users() map[string]int {
	return map[string]int{"ann": 1, "bob": 2, "cat": 3, "dan": 4, "eve": 5, "fay": 6, "gus": 7, "hal": 8}
}

func ranks() map[string]int {
	return map[string]int{"ann": 1, "bob": 1, "cat": 1, "dan": 2, "eve": 2, "fay": 2}
}

func zeros() map[string]float64 {
	negative := math.Copysign(0, -1)
	return map[string]float64{"a": negative, "b": 0, "c": negative, "d": 0, "e": negative, "f": 0}
}

var Variants = []variant.Variant{
	{Name: "FirstThree", Run: func() string { return fmt.Sprint(FirstThree(users())) }},
	{Name: "RankByValue", Run: func() string { return fmt.Sprint(RankByValue(ranks())) }},
	{Name: "SortedValues", Run: func() string { return fmt.Sprint(SortedValues(zeros())) }},
	{Name: "SortedIDs", Run: func() string { return fmt.Sprint(SortedIDs(users())) }},
	{Name: "UnsortedIDs", Run: func() string { return fmt.Sprint(UnsortedIDs(users())) }},
}
