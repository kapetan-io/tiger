package fixture

import (
	"fmt"
	"maps"
	"slices"
	"sort"
)

// A copy of the slice header shares the backing array, so printing it before
// the sort shows the map's order.
func headerCopy(m map[string]int) {
	s := make([]string, 0, len(m))
	t := s
	for k := range m { // want `TS-T02`
		s = append(s, k)
	}
	fmt.Println(t[:len(m)])
	sort.Strings(s)
}

func arrayBacked(m map[string]int) {
	var arr [8]string
	s := arr[:0]
	for k := range m { // want `TS-T02`
		s = append(s, k)
	}
	fmt.Println(arr)
	sort.Strings(s)
}

func headerCopyAppendSeq(m map[string]int) {
	s := make([]string, 0, len(m))
	t := s
	s = slices.AppendSeq(s, maps.Keys(m)) // want `TS-T02`
	fmt.Println(t[:len(m)])
	sort.Strings(s)
}

// The continue skips the step, so the next visit overwrites the slot.
func indexedContinue(m map[string]int) []string {
	s := make([]string, len(m)+1)
	i := 0
	for k, v := range m { // want `TS-T02`
		s[i] = k
		if v == 0 {
			continue
		}
		i++
	}
	sort.Strings(s)
	return s
}

// The break on another entry skips the assignment.
func flagThenBreak(m map[string]int) bool {
	a := false
	for _, v := range m { // want `TS-T02`
		if v == 1 {
			a = true
		}
		if v == 2 {
			break
		}
	}
	return a
}

func flagThenReturn(m map[string]int) bool {
	a := false
	for _, v := range m { // want `TS-T02`
		if v == 1 {
			a = true
		}
		if v == 2 {
			return false
		}
	}
	return a
}

// These stay order-free: every exit applies the only outcome the body has.
func flagAndBreak(m map[string]int) bool {
	a := false
	for _, v := range m {
		if v == 1 {
			a = true
			break
		}
	}
	return a
}

func returnTrueOnly(m map[string]int) bool {
	for _, v := range m {
		if v == 1 {
			return true
		}
	}
	return false
}

func sameFlagTwoPlaces(m map[string]int) bool {
	a := false
	for _, v := range m {
		if v == 1 {
			a = true
		}
		if v == 2 {
			a = true
			break
		}
	}
	return a
}

func freshLiteral(m map[string]int) []string {
	s := []string{}
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

// A nested exit only sees outcomes set before it, not ones set later.
func exitBeforeFlag(m map[string]int) bool {
	found := false
	for _, v := range m { // want `TS-T02`
		if v > 0 {
			break
		}
		found = true
	}
	return found
}

func returnBeforeFlag(m map[string]int) (ok bool) {
	for _, v := range m { // want `TS-T02`
		if v > 0 {
			return
		}
		ok = true
	}
	return
}

// A boolean declared in the body is not an outcome of the loop.
func localFlag(m map[string]int) bool {
	for _, v := range m {
		ok := false
		if v > 3 {
			ok = true
		}
		if ok {
			return true
		}
	}
	return false
}

var sharedBuf = make([]string, 0, 8)

func allocShared() ([]string, error) { return sharedBuf[:0], nil }

// A slice from a multi-value call may share storage.
func aliasFromMulti(m map[string]int) string {
	s, _ := allocShared()
	for k := range m { // want `TS-T02`
		s = append(s, k)
	}
	out := fmt.Sprint(sharedBuf[:len(m)])
	sort.Strings(s)
	return out
}
