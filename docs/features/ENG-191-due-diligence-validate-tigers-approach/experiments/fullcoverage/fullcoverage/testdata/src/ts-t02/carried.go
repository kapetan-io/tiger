// Item 4: a read of state an earlier visit wrote lets visit order decide
// what the loop does, even when every statement is an allowlisted shape.
package fixture

import (
	"maps"
	"slices"
)

type entry struct{ name string }

// firstThree is experiments/maporder's FirstThree: the counter picks which
// three entries are copied. tiger passes it.
func firstThree(m map[string]int) map[string]int {
	out := map[string]int{}
	n := 0
	for k, v := range m { // want `TS-T02: this loop reads state it changed`
		n++
		if n <= 3 {
			out[k] = v
		}
	}
	return out
}

// firstThreeBySize gates on the output's own size instead of a counter.
func firstThreeBySize(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m { // want `TS-T02: this loop reads state it changed`
		if len(out) < 3 {
			out[k] = v
		}
	}
	return out
}

// visitIndex writes each entry's visit position.
func visitIndex(m map[string]int) map[string]int {
	out := map[string]int{}
	n := 0
	for k := range m { // want `TS-T02: this loop reads state it changed`
		n++
		out[k] = n
	}
	return out
}

// visitIndexThroughLocal copies the counter into a fresh local first.
func visitIndexThroughLocal(m map[string]int) map[string]int {
	out := map[string]int{}
	n := 0
	for k := range m { // want `TS-T02: this loop reads state it changed`
		n++
		position := n
		out[k] = position
	}
	return out
}

// weightedSum weights each value by its visit position.
func weightedSum(m map[string]int) int {
	total, n := 0, 0
	for _, v := range m { // want `TS-T02: this loop reads state it changed`
		n++
		total += n * v
	}
	return total
}

// pruneWhileLarge deletes entries until the map is small; which survive
// depends on visit order.
func pruneWhileLarge(m map[string]int) {
	for k := range m { // want `TS-T02: this loop reads state it changed`
		if len(m) > 3 {
			delete(m, k)
		}
	}
}

// groupByValue appends each key into a per-value slice, so every group's
// slice is in map order. tiger passes it: the write is into a map.
func groupByValue(m map[string]int) map[int][]string {
	groups := map[int][]string{}
	for k, v := range m { // want `TS-T02: this loop reads state it changed`
		groups[v] = append(groups[v], k)
	}
	return groups
}

// skipOnceFound stops copying after a flag is set. Order-dependent.
func skipOnceFound(m map[string]int, target int) map[string]int {
	out := map[string]int{}
	found := false
	for k, v := range m { // want `TS-T02: this loop reads state it changed`
		if found {
			continue
		}
		if v == target {
			found = true
		}
		out[k] = v
	}
	return out
}

// lastName writes a field of an outer struct: the last entry visited wins.
// tiger passes it; its local-assignment arm skipped non-identifier targets.
func lastName(m map[string]int) entry {
	var last entry
	for k := range m { // want `TS-T02: this loop's body may depend on the order`
		last.name = k
	}
	return last
}

// lastInSlot writes slot 0 of an outer slice: the last entry visited wins.
func lastInSlot(m map[string]int) []string {
	slots := make([]string, 1)
	for k := range m { // want `TS-T02: this loop's body may depend on the order`
		slots[0] = k
	}
	return slots
}

// --- still accepted: carried state is written but never read ---

// countAndCopy counts entries beside a copy; nothing reads the count inside
// the loop.
func countAndCopy(m map[string]int) (map[string]int, int) {
	out := map[string]int{}
	n := 0
	for k, v := range m {
		n++
		out[k] = v
	}
	return out, n
}

// maxValueIf is the min/max if-form; it reads its own target by design.
func maxValueIf(m map[string]int) int {
	best := 0
	for _, v := range m {
		if v > best {
			best = v
		}
	}
	return best
}

// tally counts values into a map keyed by value.
func tally(m map[string]int) map[int]int {
	counts := map[int]int{}
	for _, v := range m {
		counts[v]++
	}
	return counts
}

// sortedFirstThree is the compliant form of firstThree.
func sortedFirstThree(m map[string]int) map[string]int {
	out := map[string]int{}
	for _, k := range slices.Sorted(maps.Keys(m))[:min(3, len(m))] {
		out[k] = m[k]
	}
	return out
}

// An alias of the written map is read through another name; the read is
// caught by the map's type.
func firstThreeThroughAlias(m map[string]int) map[string]int {
	out := map[string]int{}
	view := out
	for k, v := range m { // want `TS-T02: this loop reads state it changed`
		if len(view) < 3 {
			out[k] = v
		}
	}
	return out
}

// A map of the same type written at a non-key index can reach any slot, so
// reading out[k] is no longer reading only this visit's own slot.
func slotSharedWithUnkeyedWrite(m map[string]int, labels map[string]string) map[string]int {
	out := map[string]int{}
	other := out
	for k, v := range m { // want `TS-T02: this loop reads state it changed`
		other[labels[k]] = v
		out[k] = out[k] + v
	}
	return out
}

// --- accepted: each visit reads only the slot it writes ---

// mergeByKey appends into the slot of the range key; no other visit
// touches that slot.
func mergeByKey(dst map[string][]int, src map[string][]int) {
	for k, vs := range src {
		dst[k] = append(dst[k], vs...)
	}
}

type holder struct{ items map[string][]int }

// cloneField copies a field of one struct into the same field of another;
// the source's field is not the written map.
func cloneField(dst, src *holder) {
	for k := range src.items {
		dst.items[k] = append([]int{}, src.items[k]...)
	}
}

// fillMissing writes a slot only when it is empty.
func fillMissing(dst map[string]int, src map[string]int) {
	for k, v := range src {
		if dst[k] == 0 {
			dst[k] = v
		}
	}
}

// The first variable of a maps.Values range is a value, and values repeat,
// so out[v] is not a slot only this visit writes. This loop happens to be
// order-free (every visit with the same v does the same thing), so the
// finding is conservative; the own-slot rule is kept to range keys.
func valuesAreNotKeys(m map[string]int) map[int]int {
	out := map[int]int{}
	for v := range maps.Values(m) { // want `TS-T02: this loop reads state it changed`
		out[v] = out[v]*10 + v
	}
	return out
}

type state struct {
	deferred, fatal bool
	owner           string
	parent          *state
	marked          bool
}

// indexedKeys collects keys by position and returns them unsorted. tiger
// passes it: slice-element writes were skipped as locals.
func indexedKeys(m map[string]int) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m { // want `TS-T02: this loop's body may depend on the order`
		keys[i] = k
		i++
	}
	return keys
}

// ownerByKey writes the key into the entry; two keys sharing one pointer
// leave whichever was visited last.
func ownerByKey(m map[string]*state) {
	for k, s := range m { // want `TS-T02: this loop's body may depend on the order`
		s.owner = k
	}
}

// markUnderMarkedParent reads a field the loop writes through another
// entry.
func markUnderMarkedParent(m map[string]*state) {
	for _, s := range m { // want `TS-T02: this loop reads state it changed`
		if s.parent != nil && !s.parent.marked {
			s.marked = true
		}
	}
}

// --- accepted: a visit writes only its own entry ---

func flagEntries(m map[string]*state) {
	for _, s := range m {
		if !s.deferred {
			s.fatal = true
		}
	}
}

type rule struct{ severity string }

func defaultSeverity(rules map[string]rule, severity string) {
	for k, r := range rules {
		if r.severity == "" {
			r.severity = severity
		}
		rules[k] = r
	}
}
