// Pins the analyzer verifies for the shapes beyond a fixed step, and for the
// sound counters it leaves to boundedloop.
package fixture

// pinnedVarint states the shift loop's variant.
func pinnedVarint(size uint64) int {
	bytes := 1
	//tiger:variant size
	for size > 0 {
		size >>= 7
		bytes++
	}
	return bytes
}

// pinnedFrames states the sideband loop's variant.
func pinnedFrames(data []byte) int {
	packets := 0
	//tiger:variant len(data)
	for len(data) > 0 {
		chunk := data
		if len(chunk) > frameMax {
			chunk = chunk[:frameMax]
		}
		data = data[len(chunk):]
		packets++
	}
	return packets
}

// pinnedQueue states the field loop's variant.
func pinnedQueue(q *queue) int {
	popped := 0
	//tiger:variant len(q.items)
	for len(q.items) > 0 {
		q.items = q.items[1:]
		popped++
	}
	return popped
}

// pinnedBacktrack states the second half of an && condition.
func pinnedBacktrack(x, y int) int {
	steps := 0
	//tiger:variant y
	for x > 0 && y > 0 {
		x--
		y--
		steps++
	}
	return steps
}

// countedWithoutPin is a sound counter, which this rule leaves alone.
func countedWithoutPin(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}
