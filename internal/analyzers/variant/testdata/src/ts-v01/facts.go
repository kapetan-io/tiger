// Unpinned TS-V01 loops where synthesis succeeds: each fires the
// TS-V01-facts reported finding, printed only under --show-facts, in exact
// pin syntax so ENG-151 can freeze it into a //tiger:variant pin by pasting
// the printed line.
package fixture

// drain shrinks pending by one every round until it is empty.
func drain(pending []int) int {
	drained := 0
	for len(pending) > 0 { // want `TS-V01: synthesized variant — //tiger:variant len\(pending\)`
		pending = pending[1:]
		drained++
	}
	return drained
}

// twoPointer narrows low toward high until they meet or a match is found.
func twoPointer(arr []int, target int) bool {
	low, high := 0, len(arr)-1
	for low < high { // want `TS-V01: synthesized variant — //tiger:variant high - low`
		if arr[low]+arr[high] == target {
			return true
		}
		low++
	}
	return false
}

// countdown ticks i down to zero.
func countdown(i int) int {
	ticks := 0
	for i > 0 { // want `TS-V01: synthesized variant — //tiger:variant i`
		i--
		ticks++
	}
	return ticks
}

// indexWalk sums every other index from i up to n.
func indexWalk(i, n int) int {
	sum := 0
	for i < n { // want `TS-V01: synthesized variant — //tiger:variant n - i`
		sum += i
		i += 2
	}
	return sum
}
