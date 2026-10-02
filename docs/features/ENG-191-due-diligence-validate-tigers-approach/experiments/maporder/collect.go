// Package maporder holds map loops where TS-T02's verdict and the loop's
// output disagree, plus the collect-then-sort shape call 8 accepts.
package maporder

import "sort"

// SortedIDs is querator's collect-then-sort shape. Its output never varies,
// and call 8 makes it pass.
func SortedIDs(m map[string]int) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// UnsortedIDs is the same loop without the sort. Its output follows map order.
func UnsortedIDs(m map[string]int) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}
