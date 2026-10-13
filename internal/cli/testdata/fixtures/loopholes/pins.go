package loopholes

// Rows is the part of a database cursor the loops use.
type Rows interface {
	Next() bool
}

// WrongWay pins a ranking that grows, because i moves away from n.
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

// ChunksParamMax slices by the length of a part that is empty when max is
// zero, so the loop can stand still.
func ChunksParamMax(data []byte, max int) int {
	count := 0
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

// DrainRefill pins len(pending), but the body appends back into it.
func DrainRefill(pending []string, refill func() []string) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		pending = pending[1:]
		drained++
		pending = append(pending, refill()...)
	}
	return drained
}

// DrainSkip pins len(pending), but continue skips the shrink.
func DrainSkip(pending []string, skip func() bool) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		if skip() {
			continue
		}
		pending = pending[1:]
		drained++
	}
	return drained
}

// CountVariant pins a variable nothing in the loop shrinks.
func CountVariant(rows Rows, remaining int) int {
	count := 0
	//tiger:variant remaining
	for rows.Next() {
		count++
	}
	return count
}
