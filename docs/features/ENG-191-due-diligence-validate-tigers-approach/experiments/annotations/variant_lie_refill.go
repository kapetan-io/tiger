package annotations

// DrainRefill makes the same claim as Drain, but the body appends back into
// pending. If refill never returns empty, the loop never ends.
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
