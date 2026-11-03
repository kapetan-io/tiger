package loopsounds


const frameMax = 65515

const _ = uint(65520 - frameMax)


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


func VarintBytes(size uint64) int {
	bytes := 1
	for size > 0 {
		size >>= 7
		bytes++
	}
	return bytes
}


func HeaderBytes(size int) int {
	bytes := 1
	size >>= 4
	for size > 0 {
		size >>= 7
		bytes++
	}
	return bytes
}


func Halvings(value int) int {
	count := 0
	for value > 0 {
		value /= 2
		count++
	}
	return count
}


type Point struct {
	X, Y int
}


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


func Reverse(items []int) {
	low, high := 0, len(items)-1
	for low < high {
		items[low], items[high] = items[high], items[low]
		low++
		high--
	}
}


func ReversePinned(items []int) {
	low, high := 0, len(items)-1
	//tiger:variant high - low
	for low < high {
		items[low], items[high] = items[high], items[low]
		low++
		high--
	}
}


func EveryOther(n int) int {
	sum := 0
	i := 0
	for i < n {
		sum += i
		i += 2
	}
	return sum
}


type Queue struct {
	Items []int
}


func Drain(queue *Queue) int {
	drained := 0
	for len(queue.Items) > 0 {
		queue.Items = queue.Items[1:]
		drained++
	}
	return drained
}


func DrainPinned(queue *Queue) int {
	drained := 0
	//tiger:variant len(queue.Items)
	for len(queue.Items) > 0 {
		queue.Items = queue.Items[1:]
		drained++
	}
	return drained
}
