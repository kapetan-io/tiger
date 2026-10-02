// Item 1, collect-then-sort: a map loop that only appends into a local slice
// passes when the slice's first use after the loop is a sort that fixes its
// order. Plain sorts do on string and integer elements; comparator sorts
// do only with a full-coverage comparator (comparators.go).
package fixture

import (
	"cmp"
	"maps"
	"slices"
	"sort"
)

type User struct {
	ID   string
	Team string
}

type Priority int

type Name string

// querator's memory store shape: collect keys, sort.Strings.
func collectIDsSortStrings(m map[string]User) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func collectIntsSortInts(m map[int]bool) []int {
	var ids []int
	for id := range m {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// Values, not keys: equal ints are indistinguishable, so slices.Sort fixes
// the order even with duplicates.
func collectValuesSlicesSort(m map[string]Priority) []Priority {
	var out []Priority
	for _, p := range m {
		if p > 0 {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func collectNamedStrings(m map[Name]int) []Name {
	var names []Name
	for n := range m {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Two targets, each sorted before any other use, beside a map write.
func collectTwoTargets(m map[string]int) ([]string, []int, map[string]bool) {
	var keys []string
	var values []int
	seen := map[string]bool{}
	for k, v := range m {
		seen[k] = true
		keys = append(keys, k)
		values = append(values, v)
	}
	sort.Strings(keys)
	slices.Sort(values)
	return keys, values, seen
}

// Floats are excluded: -0 and +0 compare equal and print differently.
func collectFloatsSortFloat64s(m map[string]float64) []float64 {
	var values []float64
	for _, v := range m { // want `TS-T02: this loop collects map entries and sorts them`
		values = append(values, v)
	}
	sort.Float64s(values)
	return values
}

func collectFloatsSlicesSort(m map[string]float64) []float64 {
	var values []float64
	for _, v := range m { // want `TS-T02: this loop collects map entries and sorts them`
		values = append(values, v)
	}
	slices.Sort(values)
	return values
}

// The call-8 counter-example: ties on Team leak map order.
func collectUsersByTeam(m map[string]User) []User {
	var users []User
	for _, u := range m { // want `TS-T02: this loop collects map entries and sorts them`
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Team < users[j].Team })
	return users
}

// Stable does not help: it preserves the map order of ties.
func collectUsersByTeamStable(m map[string]User) []User {
	var users []User
	for _, u := range m { // want `TS-T02: this loop collects map entries and sorts them`
		users = append(users, u)
	}
	sort.SliceStable(users, func(i, j int) bool { return users[i].Team < users[j].Team })
	return users
}

// Expected false positive: an index comparator over whole strings is
// total, but sort.Slice comparators are not analyzed.
func collectIDsSortSlice(m map[string]User) []string {
	var ids []string
	for id := range m { // want `TS-T02: this loop collects map entries and sorts them`
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

type byID []User

func (s byID) Len() int { return len(s) }
func (s byID) Less(i, j int) bool {
	return cmp.Or(cmp.Compare(s[i].ID, s[j].ID), cmp.Compare(s[i].Team, s[j].Team)) < 0
}
func (s byID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Expected false positive: Less is total, but sort.Sort's Less is not
// analyzed.
func collectUsersSortSort(m map[string]User) []User {
	var users []User
	for _, u := range m { // want `TS-T02: this loop collects map entries and sorts them`
		users = append(users, u)
	}
	sort.Sort(byID(users))
	return users
}

// Full coverage: compares every field of User, so ties are equal values.
func collectUsersTotal(m map[string]User) []User {
	var users []User
	for _, u := range m {
		users = append(users, u)
	}
	slices.SortFunc(users, func(a, b User) int {
		return cmp.Or(cmp.Compare(a.Team, b.Team), cmp.Compare(a.ID, b.ID))
	})
	return users
}

// The strict rewrite: build in key order, then any sort is deterministic.
func usersByTeamRewrite(m map[string]User) []User {
	var users []User
	for _, id := range slices.Sorted(maps.Keys(m)) {
		users = append(users, m[id])
	}
	sort.SliceStable(users, func(i, j int) bool { return users[i].Team < users[j].Team })
	return users
}

// Used before it is sorted.
func collectUsedBeforeSort(m map[string]User) (string, []string) {
	var ids []string
	for id := range m { // want `TS-T02: this loop appends to a slice`
		ids = append(ids, id)
	}
	first := ids[0]
	sort.Strings(ids)
	return first, ids
}

// break makes which keys are collected depend on visit order.
func collectFirstThreeBreak(m map[string]User) []string {
	var ids []string
	for id := range m { // want `TS-T02: this loop appends to a slice`
		ids = append(ids, id)
		if len(ids) == 3 {
			break
		}
	}
	sort.Strings(ids)
	return ids
}

// A counter carried across iterations gates the append: same leak, no break.
func collectFirstThreeCounter(m map[string]User) []string {
	var ids []string
	n := 0
	for id := range m { // want `TS-T02: this loop reads state it changed`
		n++
		if n <= 3 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

type ranked struct {
	name string
	rank int
}

// The counter is in the element itself: every field is compared, yet the
// ranks follow visit order.
func collectVisitRank(m map[string]User) []ranked {
	var out []ranked
	n := 0
	for id := range m { // want `TS-T02: this loop reads state it changed`
		n++
		out = append(out, ranked{id, n})
	}
	slices.SortFunc(out, func(a, b ranked) int {
		return cmp.Or(cmp.Compare(a.name, b.name), cmp.Compare(a.rank, b.rank))
	})
	return out
}

// Calls in map order are a side effect the sort cannot undo.
func collectWithCall(m map[string]User, f func(string) string) []string {
	var ids []string
	for id := range m { // want `TS-T02: this loop appends to a slice`
		ids = append(ids, f(id))
	}
	sort.Strings(ids)
	return ids
}

// Expected false positive: a sort helper is not recognized as a sort.
func sortHelper(ids []string) { sort.Strings(ids) }

func collectThenHelper(m map[string]User) []string {
	var ids []string
	for id := range m { // want `TS-T02: this loop appends to a slice`
		ids = append(ids, id)
	}
	sortHelper(ids)
	return ids
}
