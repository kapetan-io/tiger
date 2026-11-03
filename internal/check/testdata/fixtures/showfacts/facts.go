// Package showfacts holds unpinned loops whose synthesized variants
// print under --show-facts in freeze-ready pin syntax, and nothing else.
package showfacts


type Recorder struct{}


func (r *Recorder) Consume(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}


func Drain(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}
