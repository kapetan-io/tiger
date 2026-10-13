// Package linecount reproduces golang.org/x/tools@v0.49.0 go/packages/internal/linecount/linecount.go:168:
// print name/count pairs by count descending; equal counts print in map order.
package linecount

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"fullcoverage/cases/variant"
)

type item struct {
	name  string
	count int
}

// Original is the shipped shape: ties on count leak map order.
func Original(m map[string]int) string {
	var buf strings.Builder
	var items []item
	for name, count := range m {
		items = append(items, item{name, count})
	}
	slices.SortFunc(items, func(x, y item) int {
		return -cmp.Compare(x.count, y.count)
	})
	for _, item := range items {
		fmt.Fprintf(&buf, "%d\t%s\n", item.count, item.name)
	}
	return buf.String()
}

// S1 ranges over the sorted keys, then the same sort made stable.
func S1(m map[string]int) string {
	var buf strings.Builder
	var items []item
	for _, name := range slices.Sorted(maps.Keys(m)) {
		items = append(items, item{name, m[name]})
	}
	slices.SortStableFunc(items, func(x, y item) int {
		return -cmp.Compare(x.count, y.count)
	})
	for _, item := range items {
		fmt.Fprintf(&buf, "%d\t%s\n", item.count, item.name)
	}
	return buf.String()
}

// S1Unstable keeps the original unstable SortFunc after ranging sorted keys: still leaks
// nothing from the map, but pdqsort reorders ties, so it is deterministic yet not key-ordered.
func S1Unstable(m map[string]int) string {
	var buf strings.Builder
	var items []item
	for _, name := range slices.Sorted(maps.Keys(m)) {
		items = append(items, item{name, m[name]})
	}
	slices.SortFunc(items, func(x, y item) int {
		return -cmp.Compare(x.count, y.count)
	})
	for _, item := range items {
		fmt.Fprintf(&buf, "%d\t%s\n", item.count, item.name)
	}
	return buf.String()
}

// S3 collects the keys, sorts them with sort.Strings (the option-A shape), then builds and
// stable-sorts the items.
func S3(m map[string]int) string {
	var buf strings.Builder
	var names []string
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	var items []item
	for _, name := range names {
		items = append(items, item{name, m[name]})
	}
	slices.SortStableFunc(items, func(x, y item) int {
		return -cmp.Compare(x.count, y.count)
	})
	for _, item := range items {
		fmt.Fprintf(&buf, "%d\t%s\n", item.count, item.name)
	}
	return buf.String()
}

// S4 keeps the map loop and makes the comparator total, ending on the key.
func S4(m map[string]int) string {
	var buf strings.Builder
	var items []item
	for name, count := range m {
		items = append(items, item{name, count})
	}
	slices.SortFunc(items, func(x, y item) int {
		return cmp.Or(-cmp.Compare(x.count, y.count), strings.Compare(x.name, y.name))
	})
	for _, item := range items {
		fmt.Fprintf(&buf, "%d\t%s\n", item.count, item.name)
	}
	return buf.String()
}

// S6 drops the item struct: sort the keys with a comparator that reads the map.
func S6(m map[string]int) string {
	var buf strings.Builder
	names := slices.Sorted(maps.Keys(m))
	slices.SortStableFunc(names, func(x, y string) int { return -cmp.Compare(m[x], m[y]) })
	for _, name := range names {
		fmt.Fprintf(&buf, "%d\t%s\n", m[name], name)
	}
	return buf.String()
}

func fixture() map[string]int {
	return map[string]int{"a.go": 10, "b.go": 10, "c.go": 10, "d.go": 7, "e.go": 7, "f.go": 3, "g.go": 3, "h.go": 3}
}

func render(f func(map[string]int) string) func() string {
	return func() string { return f(fixture()) }
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "S1", Run: render(S1)},
	{Name: "S1Unstable", Run: render(S1Unstable)},
	{Name: "S3", Run: render(S3)},
	{Name: "S4", Run: render(S4)},
	{Name: "S6", Run: render(S6)},
}
