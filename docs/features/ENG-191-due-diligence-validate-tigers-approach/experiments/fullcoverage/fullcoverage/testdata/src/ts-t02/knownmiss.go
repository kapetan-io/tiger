// TS-T02 coverage gaps. tiger's corpus held one known miss here,
// slices.Collect(maps.Keys(m)); the call 8 iterator check now catches it, so
// its expectation changed from silent to a finding. The two loops after it
// still pass and still depend on map order: they are the gaps this
// prototype leaves open, kept here so a fix shows up as a changed verdict.
package fixture

import (
	"fmt"
	"maps"
	"slices"
)

// unsortedCollectKnownMiss collects m's keys via slices.Collect(maps.Keys(m))
// without sorting them, then ranges over the resulting slice. tiger misses
// it; the iterator check does not.
func unsortedCollectKnownMiss(m map[string]int) []int {
	var result []int
	for _, k := range slices.Collect(maps.Keys(m)) { // want `TS-T02: this iterator yields map entries`
		result = append(result, m[k])
	}
	return result
}

// invertLastWriterWinsKnownMiss inverts m. Two keys with the same value
// write the same slot, and the one visited last wins.
//
// known-miss: the map-write arm checks what a write reads, not whether two
// visits can write the same key.
func invertLastWriterWinsKnownMiss(m map[string]int) map[int]string {
	inverse := map[int]string{}
	for k, v := range m {
		inverse[v] = k
	}
	return inverse
}

// labelWithCallKnownMiss calls label once per entry, in map order, while
// writing a map. If label prints, the output follows map order.
//
// known-miss: the map-write arm does not require its value to be call-free.
func labelWithCallKnownMiss(m map[string]int) map[string]string {
	labels := map[string]string{}
	for k, v := range m {
		labels[k] = fmt.Sprint(v)
	}
	return labels
}
