// Known false findings: each loop's output does not depend on map order, but
// the shape is outside the allowlist, so TS-T02 fires. Clearing these needs a
// proof about the data (a key that is unique, an index that is distinct) or
// alias analysis of a field. The module-cache scan found no site that a
// comparator held in a variable or a sort.Sort would clear.
package fixture

import "sort"

type pair struct {
	Key   string
	Value any
}

// The comparator covers the key, not the whole element, and keys are unique
// only because they came from a map.
func uniqueByInvariant(m map[string]any) []pair {
	items := make([]pair, len(m))
	i := 0
	for k, v := range m { // want `TS-T02: this sort can find two different map entries equal`
		items[i] = pair{k, v}
		i++
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

type names struct{ list []string }

// The slice is a field, so something else may hold it.
func (n *names) fill(m map[string]int) {
	n.list = make([]string, len(m))
	i := 0
	for k := range m { // want `TS-T02: this loop's body may depend on the order`
		n.list[i] = k
		i++
	}
	sort.Strings(n.list)
}

// Each value is a distinct index by the caller's invariant.
func permute(m map[string]int, out []string) {
	for k, v := range m { // want `TS-T02: this loop's body may depend on the order`
		out[v] = k
	}
}

type keyedEntry struct{ Key string }

// The value carries the key it is stored under, so this deletes only the
// current entry, but the rule only knows the range key.
func dropByValueKey(m map[string]*keyedEntry) {
	for _, e := range m { // want `TS-T02`
		delete(m, e.Key)
	}
}
