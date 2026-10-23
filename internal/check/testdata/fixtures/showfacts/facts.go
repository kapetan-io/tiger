// Package showfacts holds unpinned loops whose synthesized variants
// print under --show-facts in freeze-ready pin syntax, and nothing else.
package showfacts

// Recorder totals entries.
type Recorder struct{}

// Consume totals pending and empties it.
func (r *Recorder) Consume(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}

// Drain consumes pending and totals it.
func Drain(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}
