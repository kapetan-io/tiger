package fixture

import (
	"fmt"
	"maps"
	"slices"
	"sort"
)

var globalNames []string

func showGlobalNames() { fmt.Println(globalNames) }

// Alias leaks: variants.

func globalDirectPrint(m map[string]int) {
	for k := range m { // want `TS-T02`
		globalNames = append(globalNames, k)
	}
	fmt.Println(globalNames)
	sort.Strings(globalNames)
}

func globalNeverSorted(m map[string]int) {
	for k := range m { // want `TS-T02`
		globalNames = append(globalNames, k)
	}
}

func globalSortedAtOnce(m map[string]int) {
	for k := range m { // want `TS-T02`
		globalNames = append(globalNames, k)
	}
	sort.Strings(globalNames)
	showGlobalNames()
}

func closureCalledAfterSortOnly(m map[string]int) {
	var keys []string
	show := func() { fmt.Println(keys) }
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sort.Strings(keys)
	show()
}

func closureDeferred(m map[string]int) {
	var keys []string
	defer func() { fmt.Println(keys) }()
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sort.Strings(keys)
}

func closureNeverCalled(m map[string]int) []string {
	var keys []string
	_ = func() { fmt.Println(keys) }
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func pointerTakenAfterLoop(m map[string]int) {
	var keys []string
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	p := &keys
	fmt.Println(*p)
	sort.Strings(keys)
}

func pointerPrintedAfterSort(m map[string]int) {
	var keys []string
	p := &keys
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println(*p)
}

func sliceAliasPrinted(m map[string]int) {
	var keys []string
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	head := keys[:]
	fmt.Println(head)
	sort.Strings(keys)
}

func sliceAliasBeforeLoopPrinted(m map[string]int) {
	keys := make([]string, 0, 8)
	head := keys[:0]
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	fmt.Println(head[:len(keys)])
	sort.Strings(keys)
}

func plainAssignAliasPrinted(m map[string]int) {
	var keys []string
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	same := keys
	fmt.Println(same)
	sort.Strings(keys)
}

func passedToFuncBeforeSort(m map[string]int, sink func([]string)) {
	var keys []string
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sink(keys)
	sort.Strings(keys)
}

func pointerPassedAsArg(m map[string]int, sink func(*[]string)) {
	var keys []string
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sink(&keys)
	sort.Strings(keys)
}

func goroutineCaptures(m map[string]int) {
	var keys []string
	go func() { fmt.Println(keys) }()
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sort.Strings(keys)
}

// Control: captured, but only read after sort, closure defined after sort.
func closureDefinedAfterSort(m map[string]int) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	show := func() { fmt.Println(keys) }
	show()
}

// Struct field holder: slice stored in a struct before sort.
type nameHolder struct{ names []string }

func fieldAliasLeak(m map[string]int) {
	var keys []string
	var h nameHolder
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	h.names = keys
	fmt.Println(h.names)
	sort.Strings(keys)
}

// Parenthesized iterator: must behave exactly as unparenthesized.

func parenCollectUnsorted(m map[string]int) []string {
	return slices.Collect((maps.Keys(m))) // want `TS-T02: this iterator`
}

func parenSortedFloats(m map[string]float64) []float64 {
	return slices.Sorted((maps.Values(m))) // want `TS-T02: this iterator`
}

func parenSortedIntValues(m map[string]int) []int {
	return slices.Sorted((maps.Values(m)))
}

func parenPassed(m map[string]int) {
	consume((maps.Keys(m))) // want `TS-T02: this iterator`
}

func parenRangeKeysAppend(m map[string]int) []string {
	var names []string
	for name := range maps.Keys(m) { // want `TS-T02: this loop appends`
		names = append(names, name)
	}
	return names
}

func parenRangeKeysAppendSorted(m map[string]int) []string {
	var names []string
	for name := range maps.Keys(m) {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func parenRangeAllPrint(m map[string]int) {
	for k, v := range maps.All(m) { // want `TS-T02`
		fmt.Println(k, v)
	}
}

func doubleParenSorted(m map[string]int) []string {
	return slices.Sorted((maps.Keys(m)))
}

func parenMapRange(m map[string]int) {
	for k := range m { // want `TS-T02`
		fmt.Println(k)
	}
}

func parenMapRangeCopy(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func parenRankByValue(m map[string]int) []string {
	return slices.SortedFunc((maps.Keys(m)), func(a, b string) int { return m[a] - m[b] }) // want `TS-T02: this iterator`
}

// Labels.

func labeledUnsortedCollect(m map[string]int) []string {
	var keys []string
outer:
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	_ = 0
	if len(keys) < 0 {
		goto outer
	}
	return keys
}

func labeledPrint(m map[string]int) {
loop:
	for k := range m { // want `TS-T02`
		fmt.Println(k)
	}
	if len(m) < 0 {
		goto loop
	}
}

func labeledContinueCollect(m map[string]int) []string {
	var keys []string
next:
	for k, v := range m { // want `TS-T02`
		if v < 0 {
			continue next
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func labeledBreakInsideCollect(m map[string]int) []string {
	var keys []string
stop:
	for k, v := range m { // want `TS-T02`
		if v < 0 {
			break stop
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func labeledNestedOuter(m map[string]int, n []string) []string {
	var keys []string
outer:
	for k := range m { // want `TS-T02`
		for _, j := range n {
			if j == k {
				continue outer
			}
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func labeledIndexed(m map[string]int) []string {
	keys := make([]string, len(m))
	i := 0
lbl:
	for k := range m {
		keys[i] = k
		i++
	}
	_ = 0
	sort.Strings(keys)
	if i < 0 {
		goto lbl
	}
	return keys
}

func labeledWriteMap(m map[string]int) map[string]int {
	out := map[string]int{}
lbl:
	for k, v := range m {
		out[k] = v
	}
	_ = 0
	if len(out) < 0 {
		goto lbl
	}
	return out
}

// Early exits.

func returnAfterWrite(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m { // want `TS-T02`
		out[k] = v
		if v > 3 {
			return out
		}
	}
	return out
}

func unconditionalBreakAfterWrite(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m { // want `TS-T02`
		out[k] = v
		break
	}
	return out
}

func unconditionalReturnAfterWrite(m map[string]int, out map[string]int) {
	for k, v := range m { // want `TS-T02`
		out[k] = v
		return
	}
}

func gotoOutAfterWrite(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m { // want `TS-T02`
		out[k] = v
		if v > 3 {
			goto done
		}
	}
done:
	return out
}

func labeledBreakAfterWrite(m map[string]int) map[string]int {
	out := map[string]int{}
stop:
	for k, v := range m { // want `TS-T02`
		out[k] = v
		if v > 3 {
			break stop
		}
	}
	return out
}

func breakFromNestedSwitchOnly(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m { // want `TS-T02`
		switch v {
		case 1:
			break
		}
		out[k] = v
	}
	return out
}

func breakInNestedLoopOnly(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m { // want `TS-T02`
		for i := 0; i < 3; i++ {
			if i == v {
				break
			}
		}
		out[k] = v
	}
	return out
}

func deleteThenBreak(m map[string]int) {
	for k, v := range m { // want `TS-T02`
		delete(m, k)
		if v > 3 {
			break
		}
	}
}

func sumThenBreak(m map[string]int) int {
	total := 0
	for _, v := range m { // want `TS-T02`
		total += v
		if total > 10 {
			break
		}
	}
	return total
}

func collectThenBreak(m map[string]int) []string {
	var keys []string
	for k, v := range m { // want `TS-T02`
		keys = append(keys, k)
		if v > 3 {
			break
		}
	}
	sort.Strings(keys)
	return keys
}

func collectThenReturnInside(m map[string]int) []string {
	var keys []string
	for k, v := range m { // want `TS-T02`
		keys = append(keys, k)
		if v > 3 {
			return keys
		}
	}
	sort.Strings(keys)
	return keys
}

func boolShortCircuit(m map[string]int) bool {
	for _, v := range m {
		if v > 3 {
			return true
		}
	}
	return false
}

// Indexed collect.

func indexedCounterAfterLoopUse(m map[string]int) ([]string, int) {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys, i
}

func indexedCounterDecl(m map[string]int) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m { // want `TS-T02`
		keys[i] = k
		i += 1
	}
	sort.Strings(keys)
	return keys
}

func indexedFloatsSorted(m map[string]float64) []float64 {
	vals := make([]float64, len(m))
	i := 0
	for _, v := range m { // want `TS-T02`
		vals[i] = v
		i++
	}
	slices.Sort(vals)
	return vals
}

func indexedDecrement(m map[string]int) []string {
	keys := make([]string, len(m))
	i := len(m) - 1
	for k := range m { // want `TS-T02`
		keys[i] = k
		i--
	}
	sort.Strings(keys)
	return keys
}

func indexedPrintedBeforeSort(m map[string]int) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m { // want `TS-T02`
		keys[i] = k
		i++
	}
	fmt.Println(keys)
	sort.Strings(keys)
	return keys
}

func indexedCounterPrinted(m map[string]int) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m { // want `TS-T02`
		keys[i] = k
		i++
		fmt.Println(i)
	}
	sort.Strings(keys)
	return keys
}

func indexedSortSliceByLen(m map[string]int) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m { // want `TS-T02`
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(a, b int) bool { return len(keys[a]) < len(keys[b]) })
	return keys
}

// Collect/AppendSeq then sort.

func collectThenSortSliceSelf(m map[string]int) []string {
	keys := slices.Collect(maps.Keys(m))
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func collectThenSortFloatSlice(m map[string]float64) []float64 {
	v := slices.Collect(maps.Values(m)) // want `TS-T02`
	sort.Float64s(v)
	return v
}

func appendSeqThenPrint(m map[string]int) []string {
	var keys []string
	keys = slices.AppendSeq(keys, maps.Keys(m)) // want `TS-T02`
	fmt.Println(keys)
	sort.Strings(keys)
	return keys
}

func collectNeverSorted(m map[string]int) []string {
	keys := slices.Collect(maps.Keys(m)) // want `TS-T02`
	return keys
}

func collectStructValuesSort(m map[string]User) []User {
	us := slices.Collect(maps.Values(m)) // want `TS-T02`
	sort.Slice(us, func(i, j int) bool { return us[i].ID < us[j].ID })
	return us
}

func collectSortSliceSameIndex(m map[string]int) []string {
	keys := slices.Collect(maps.Keys(m)) // want `TS-T02`
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[i] })
	return keys
}

func collectSortSliceFloats(m map[string]float64) []float64 {
	v := make([]float64, 0)
	for _, x := range m { // want `TS-T02`
		v = append(v, x)
	}
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	return v
}

func collectSortSliceOtherSlice(m map[string]int, other []string) []string {
	var keys []string
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return other[i] < other[j] })
	return keys
}

func collectSortSliceMixedIndex(m map[string]int) []string {
	var keys []string
	for k := range m { // want `TS-T02`
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[j] < keys[i] || keys[i] == "" })
	return keys
}

func collectSortSliceStable(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func unlabeledContinueCollect(m map[string]int) []string {
	var keys []string
	for k, v := range m {
		if v < 0 {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
