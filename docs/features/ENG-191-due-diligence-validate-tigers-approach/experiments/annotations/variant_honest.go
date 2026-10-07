package annotations

// Drain removes one item per pass. The variant claims len(pending) shrinks
// every pass, and the body's pending = pending[1:] is a move tiger verifies.
func Drain(pending []string) int {
	drained := 0
	//tiger:variant len(pending)
	for len(pending) > 0 {
		pending = pending[1:]
		drained++
	}
	return drained
}
