// A loop TS-V01 verified a pin for is accepted by TS-S02, whether the
// developer pinned it or tiger found the variant.
package fixture

// closeIn walks two cursors toward each other.
func closeIn(items []int) int {
	low, high, swaps := 0, len(items)-1, 0
	//tiger:variant high - low
	for low < high {
		items[low], items[high] = items[high], items[low]
		low++
		high--
		swaps++
	}
	return swaps
}

// shiftOut shifts the size down seven bits per pass.
func shiftOut(size uint64) int {
	passes := 0
	for size > 0 {
		size >>= 7
		passes++
	}
	return passes
}
