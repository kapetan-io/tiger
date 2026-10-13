// The partial-comparator finding names the rewrite: sorted keys, then a
// stable sort.
package fixture

import (
	"cmp"
	"slices"
)

type scoreRow struct {
	name  string
	count int
}

func sortMessageNamesRewrite(m map[string]int) []scoreRow {
	var rows []scoreRow
	for name, count := range m { // want `slices\.Sorted\(maps\.Keys\(m\)\), then slices\.SortStableFunc`
		rows = append(rows, scoreRow{name, count})
	}
	slices.SortFunc(rows, func(a, b scoreRow) int { return cmp.Compare(a.count, b.count) })
	return rows
}
