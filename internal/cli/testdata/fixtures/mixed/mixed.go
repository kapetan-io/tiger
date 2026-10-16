// Package mixed exercises tiger pin's refusal and unexported policies.
package mixed

// Good drains pending.
func Good(pending []int) int {
	total := 0
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total
}

// Bad carries a variant pin that its loop does not shrink.
func Bad(pending []int, names []string) int {
	total := 0
	//tiger:variant len(names)
	for len(pending) > 0 {
		total += pending[0]
		pending = pending[1:]
	}
	return total + len(names)
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
