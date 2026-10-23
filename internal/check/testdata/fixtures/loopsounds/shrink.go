package loopsounds

// frameMax is the most payload one sideband packet carries.
const frameMax = 65515

const _ = uint(65520 - frameMax)

// Frames splits data into packets, the way git-server's sideband writer does.
func Frames(data []byte) int {
	packets := 0
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

// VarintBytes counts the bytes a varint takes, shifting seven bits per pass.
func VarintBytes(size uint64) int {
	bytes := 1
	for size > 0 {
		size >>= 7
		bytes++
	}
	return bytes
}

// HeaderBytes counts a signed size's continuation bytes.
func HeaderBytes(size int) int {
	bytes := 1
	size >>= 4
	for size > 0 {
		size >>= 7
		bytes++
	}
	return bytes
}

// Halvings counts how often a positive number halves before it reaches zero.
func Halvings(value int) int {
	count := 0
	for value > 0 {
		value /= 2
		count++
	}
	return count
}

// Point is a position in an edit graph.
type Point struct {
	X, Y int
}

// Backtrack walks two cursors toward the origin, the way a Myers diff does.
func Backtrack(from Point) int {
	x, y := from.X, from.Y
	steps := 0
	for x > 0 && y > 0 {
		steps++
		x--
		y--
	}
	return steps
}

// Reverse swaps from both ends until the two cursors meet.
func Reverse(items []int) {
	low, high := 0, len(items)-1
	for low < high {
		items[low], items[high] = items[high], items[low]
		low++
		high--
	}
}

// ReversePinned states the same loop's variant.
func ReversePinned(items []int) {
	low, high := 0, len(items)-1
	//tiger:variant high - low
	for low < high {
		items[low], items[high] = items[high], items[low]
		low++
		high--
	}
}

// EveryOther steps by two toward n from inside the body.
func EveryOther(n int) int {
	sum := 0
	i := 0
	for i < n {
		sum += i
		i += 2
	}
	return sum
}

// Queue holds items waiting to be handled.
type Queue struct {
	Items []int
}

// Drain pops the queue's items one at a time.
func Drain(queue *Queue) int {
	drained := 0
	for len(queue.Items) > 0 {
		queue.Items = queue.Items[1:]
		drained++
	}
	return drained
}

// DrainPinned states the same loop's variant.
func DrainPinned(queue *Queue) int {
	drained := 0
	//tiger:variant len(queue.Items)
	for len(queue.Items) > 0 {
		queue.Items = queue.Items[1:]
		drained++
	}
	return drained
}
