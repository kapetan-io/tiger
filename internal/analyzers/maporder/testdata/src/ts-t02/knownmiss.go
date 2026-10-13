// The five documented TS-T02 misses (ADR-0021). Each function passes
// although its output depends on map order, and stays here so closing a gap
// shows up as a changed verdict. slices.Collect(maps.Keys(m)) is not one of
// them: the iterator check fires on it.
package fixture

import (
	"fmt"
	"reflect"

	expmaps "golang.org/x/exp/maps"
)

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

// expKeysKnownMiss ranges over golang.org/x/exp/maps.Keys, which returns a
// slice in map order.
//
// known-miss: only the standard library's maps.Keys, maps.Values and
// maps.All are recognized as map iterators.
func expKeysKnownMiss(m map[string]int) []int {
	var result []int
	for _, k := range expmaps.Keys(m) {
		result = append(result, m[k])
	}
	return result
}

// reflectKeysKnownMiss ranges over reflect.Value.MapKeys, which returns the
// keys in map order.
//
// known-miss: reflect.Value.MapKeys is not recognized as a map iterator.
func reflectKeysKnownMiss(m map[string]int) []int {
	var result []int
	for _, k := range reflect.ValueOf(m).MapKeys() {
		result = append(result, m[k.String()])
	}
	return result
}

// typeParamRangeKnownMiss ranges over a value whose type is a type
// parameter constrained to a map.
//
// known-miss: the range-type check looks at the underlying type, and a type
// parameter's underlying type is its constraint interface, not a map.
func typeParamRangeKnownMiss[M ~map[string]int](m M) []string {
	var result []string
	for k := range m {
		result = append(result, k)
	}
	return result
}
