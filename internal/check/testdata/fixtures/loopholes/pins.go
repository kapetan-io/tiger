package loopholes


type Rows interface {
	Next() bool
}


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


func CountVariant(rows Rows, remaining int) int {
	count := 0
	//tiger:variant remaining
	for rows.Next() {
		count++
	}
	return count
}
