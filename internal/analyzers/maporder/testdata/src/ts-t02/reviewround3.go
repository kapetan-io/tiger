package fixture

import (
	"maps"
	"slices"
	"sort"
	"strings"
)

var shared = map[string]int{"a": 1, "b": 2, "c": 3}

// A continue between the collect and the sort leaves the slice unsorted for
// the rest of the enclosing loop.
func continueBeforeSort(skip bool) []string {
	var out []string
	for round := 0; round < 1; round++ {
		for k := range shared { // want `TS-T02`
			out = append(out, k)
		}
		if skip {
			continue
		}
		sort.Strings(out)
	}
	return out
}

func breakBeforeSort(skip bool) []string {
	var out []string
	for round := 0; round < 1; round++ {
		out = slices.Collect(maps.Keys(shared)) // want `TS-T02`
		if skip {
			break
		}
		sort.Strings(out)
	}
	return out
}

// A return that does not mention the slice cannot expose it.
func returnBeforeSort(m map[string]int, err error) ([]string, error) {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// Deleting an entry the range has not reached yet changes which are visited.
func deleteOther(m map[string]int) []string {
	var out []string
	for k := range m { // want `TS-T02`
		out = append(out, k)
		delete(m, "e")
	}
	sort.Strings(out)
	return out
}

// Inserting while ranging may or may not be visited.
func insertWhileRanging(m map[string]int) int {
	n := 0
	for k := range m { // want `TS-T02`
		n++
		m[k+"x"] = 1
	}
	return n
}

// Writing a different map at a computed key is fine.
func lowerKeys(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		out[strings.ToLower(k)] += v
	}
	return out
}

func deleteCurrent(m map[string]int) {
	for k, v := range m {
		if v == 0 {
			delete(m, k)
		}
	}
}
