// Documented TS-T02 coverage gaps: order escaping a map without the escape
// passing through a RangeStmt whose range target is the map itself, which is
// the only shape maporder's syntactic check inspects.
package fixture

import (
	"maps"
	"slices"
)

// unsortedCollectKnownMiss collects m's keys via slices.Collect(maps.Keys(m))
// without sorting them, then ranges over the resulting slice. The range
// target is a slice, not a map, so maporder's range-type check never sees
// it — even though the slice's element order still depends on the map's
// internal iteration order.
//
// known-miss: maporder only inspects RangeStmt nodes whose range target
// types as a map; order escaping through an intermediate unsorted slice
// collected from a map is invisible to this syntactic check.
func unsortedCollectKnownMiss(m map[string]int) []int {
	var result []int
	for _, k := range slices.Collect(maps.Keys(m)) {
		result = append(result, m[k])
	}
	return result
}
