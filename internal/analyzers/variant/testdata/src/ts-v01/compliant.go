// The TS-V01 compliant shapes: loops out of variant's scope (structurally
// terminating ranges, fixed-count counters, and the for{} event-loop shape
// that boundedloop owns), the explicit counter-cap rewrite this analyzer's
// own findings recommend, and a pinned variant the analyzer verifies.
package fixture

const tryLimit = 5

// sumSlice ranges over a slice, bounded by its own length — out of scope.
func sumSlice(entries []int) int {
	total := 0
	for _, entry := range entries {
		total += entry
	}
	return total
}

// countUp is fixed-count: Post advances i and i appears in Cond — out of
// scope, boundedloop's territory.
func countUp(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}

// pollForever is the for{} event-loop shape — no Cond, so this analyzer
// leaves it entirely to boundedloop's TS-S02/TS-S03.
func pollForever(shutdown <-chan struct{}) int {
	count := 0
	for {
		select {
		case <-shutdown:
			return count
		default:
			count++
		}
	}
}

// drainWithCap is the rewrite this analyzer's own findings recommend: an
// explicit iteration cap whose own counter is a fixed-count loop, with an
// assert on exhaustion standing in for a termination argument nobody wrote
// down.
func drainWithCap(pending []int) int {
	drained := 0
	for tries := 0; tries < tryLimit; tries++ {
		if len(pending) == 0 {
			break
		}
		pending = pending[1:]
		drained++
	}
	if len(pending) != 0 {
		panic("did not drain within tryLimit")
	}
	return drained
}

// drainPinned pins the variant the analyzer would synthesize unaided; the
// pin verifies, so pinning it is a no-op paste, not a new claim.
func drainPinned(pending []int) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		pending = pending[1:]
		drained++
	}
	return drained
}
