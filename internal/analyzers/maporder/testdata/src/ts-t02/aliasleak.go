package fixture

import (
	"fmt"
	"maps"
	"slices"
	"sort"
)

var leakedKeys []string

func emitLeakedKeys() { fmt.Println(leakedKeys) }

func leakThroughGlobal(m map[string]int) {
	for k := range m { // want `TS-T02`
		leakedKeys = append(leakedKeys, k)
	}
	emitLeakedKeys()
	sort.Strings(leakedKeys)
}

func leakThroughClosure(m map[string]int) {
	var keys []string
	show := func() { fmt.Println(keys) }
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	show()
	sort.Strings(keys)
}

func leakThroughPointer(m map[string]int) {
	var keys []string
	p := &keys
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	fmt.Println(*p)
	sort.Strings(keys)
}

func localSortedStillPasses(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func parenthesizedSorted(m map[string]int) []string {
	return slices.Sorted((maps.Keys(m)))
}

func parenthesizedAll(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range maps.All(m) {
		out[k] = v
	}
	return out
}

func labeledCollect(m map[string]int) []string {
	var keys []string
again:
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		goto again
	}
	return keys
}

func breakAfterWrite(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m { // want `TS-T02`
		out[k] = v
		if v > 3 {
			break
		}
	}
	return out
}
