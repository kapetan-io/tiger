// The TS-V01 failure modes: no linear ranking exists at all, the only
// decrease is conditional, a continue can skip the decrease, the recognized
// condition pairs with an unrecognized decrease, a pinned variant the body
// invalidates, and a pin on a loop that needs no variant.
package fixture

type node struct {
	val  int
	next *node
}

// findTail walks a linked list to its end. Termination depends on the list
// being acyclic, a fact no linear ranking over locals can see, so the
// analyzer's closed synthesis set has nothing to offer here.
func findTail(n *node) *node {
	for n != nil { // want `TS-V01: this loop has no synthesized or pinned variant`
		if n.next == nil {
			return n
		}
		n = n.next
	}
	return n
}

// drainConditional only shrinks pending when extra reports true, so the
// synthesizer's required unconditional top-level decrease is missing.
func drainConditional(pending []int, extra func() bool) int {
	drained := 0
	for len(pending) > 0 { // want `TS-V01: this loop has no synthesized or pinned variant`
		if extra() {
			pending = pending[1:]
		}
		drained++
	}
	return drained
}

// drainContinue skips the shrink on some rounds via continue, so a back
// edge can bypass the decrease entirely.
func drainContinue(pending []int, skip func() bool) int {
	drained := 0
	for len(pending) > 0 { // want `TS-V01: this loop has no synthesized or pinned variant`
		if skip() {
			continue
		}
		pending = pending[1:]
		drained++
	}
	return drained
}

// binarySearch's condition low < high is recognized, but the analyzer's
// closed decrease set does not include `low = mid + 1` / `high = mid`
// assignments — a deliberate synthesis limit. The exit is the counter-cap
// rewrite the finding names (or a future analyzer wave), never a directive.
func binarySearch(arr []int, target int) int {
	low, high := 0, len(arr)
	for low < high { // want `TS-V01: this loop has no synthesized or pinned variant`
		mid := (low + high) / 2
		if arr[mid] == target {
			return mid
		}
		if arr[mid] < target {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return -1
}

// drainPinnedAppend pins a variant the analyzer cannot verify: the body
// appends back into the measured slice, so len(pending) is not proven to
// shrink — an unverifiable pin is blocking even though the loop happens to
// terminate whenever refill eventually returns nothing.
func drainPinnedAppend(pending []int, refill func() []int) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 { // want `TS-V01: the pinned variant //tiger:variant len\(pending\) cannot be verified`
		pending = pending[1:]
		drained++
		pending = append(pending, refill()...)
	}
	return drained
}

// rangeWithPin is structurally terminating: a pin here states a fact the
// analyzer never checks, so the pin itself is the finding.
func rangeWithPin(items []int) int {
	total := 0
	//tiger:variant len(items)
	for _, item := range items { // want `TS-V01: this loop needs no variant`
		total += item
	}
	return total
}
