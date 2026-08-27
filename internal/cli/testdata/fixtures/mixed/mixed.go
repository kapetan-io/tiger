// Package mixed exercises tiger pin's refusal and unexported policies.
package mixed

import "os"

// Good reads the configured home directory.
func Good() string {
	return os.Getenv("TIGER_HOME")
}

// Bad carries a pin that disagrees with its computed effects.
//
//tiger:effects none
func Bad() string {
	return os.Getenv("TIGER_HOME")
}

// Panics carries a blocking finding.
func Panics() {
	panic("boom")
}

// count tallies pending.
func count(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}
