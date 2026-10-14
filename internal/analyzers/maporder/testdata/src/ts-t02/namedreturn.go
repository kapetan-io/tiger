package fixture

import "sort"

// A bare return hands back the named result before the sort.
func namedResultEarlyReturn(m map[string]int, early bool) (keys []string) {
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	if early {
		return
	}
	sort.Strings(keys)
	return
}

// A named result that is not the collected slice cannot expose it.
func otherNamedResult(m map[string]int, early bool) (n int) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	if early {
		return
	}
	sort.Strings(keys)
	return len(keys)
}
