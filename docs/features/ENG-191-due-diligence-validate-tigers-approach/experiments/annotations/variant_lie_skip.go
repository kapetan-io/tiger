package annotations

// DrainSkip makes the same claim, but continue skips the shrink. If skip
// always returns true, the loop never ends.
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
