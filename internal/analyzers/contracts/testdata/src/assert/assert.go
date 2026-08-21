// Package assert is a minimal stand-in for the real assert package, here
// only so a ts-v03 fixture can demonstrate a cross-package callee: a
// requires pin the contracts analyzer cannot see because it only resolves
// same-package call sites.
package assert

// Ok panics when cond is false.
func Ok(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

// Positive requires a positive count and panics otherwise — the runtime
// assertion TS-A02 always provides, and the only enforcement a
// cross-package caller gets from this pin.
//
//tiger:requires n > 0
func Positive(n int) int {
	if n <= 0 {
		panic("n must be positive")
	}
	return n
}
