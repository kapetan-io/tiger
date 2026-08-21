// Package showfacts holds unpinned exported functions whose computed
// facts — effect sets, frames, a synthesized variant — print under
// --show-facts in freeze-ready pin syntax, and nothing else.
package showfacts

import "os"

// Recorder accumulates entries.
type Recorder struct {
	log []byte
}

// Append records one entry.
func (r *Recorder) Append(entry byte) {
	r.log = append(r.log, entry)
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

// Home reads the configured home directory.
func Home() string {
	return os.Getenv("TIGER_HOME")
}
