// Package variants tries //tiger:variant expressions other than len(pending):
// five tiger verifies, one false claim it rejects, and two true claims it can't
// prove.
package variants

type Queue struct{ Items []int }

// 1. A counter that counts down.
func Countdown(i int) int {
	ticks := 0
	//tiger:variant i
	for i > 0 {
		i--
		ticks++
	}
	return ticks
}

// 2. The gap between an index and its end, stepping by 2.
func EveryOther(n int) int {
	sum := 0
	i := 0
	//tiger:variant n - i
	for i < n {
		sum += i
		i += 2
	}
	return sum
}

// 3. Two pointers closing in from both ends.
func Reverse(s []int) {
	low, high := 0, len(s)-1
	//tiger:variant high - low
	for low < high {
		s[low], s[high] = s[high], s[low]
		low++
		high--
	}
}

// 4. A stack popped from the end.
func PopAll(stack []int) int {
	popped := 0
	//tiger:variant len(stack)
	for len(stack) > 0 {
		stack = stack[:len(stack)-1]
		popped++
	}
	return popped
}

// 5. A slice reached through a struct field.
func DrainQueue(q *Queue) int {
	drained := 0
	//tiger:variant len(q.Items)
	for len(q.Items) > 0 {
		q.Items = q.Items[1:]
		drained++
	}
	return drained
}

// 6. False: the gap grows because i moves the wrong way.
func WrongWay(n int) int {
	steps := 0
	i := 0
	//tiger:variant n - i
	for i < n {
		i--
		steps++
	}
	return steps
}

// 7. True but unprovable: a shift halves size, and tiger knows no shift move.
func Varint(size uint64) int {
	bytes := 0
	//tiger:variant size
	for size > 0 {
		size >>= 7
		bytes++
	}
	return bytes
}

// 8. True but unprovable: a step of len(chunk) instead of a fixed number.
func Chunks(data []byte, max int) int {
	count := 0
	//tiger:variant len(data)
	for len(data) > 0 {
		chunk := data
		if len(chunk) > max {
			chunk = chunk[:max]
		}
		data = data[len(chunk):]
		count++
	}
	return count
}
