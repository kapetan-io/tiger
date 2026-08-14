// Package assert is a corpus miniature of the spec's canonical assert
// package. Its API — Ok(cond bool, ...) and Equal(got, want T) — is dialect
// infrastructure that the specification itself fixes, so sametypeparams
// treats this package (and its external test package) as exempt.
package assert

// Ok panics when cond is false. The bool parameter is the sanctioned
// assert-package shape, not a TS-N08 finding.
func Ok(cond bool, args ...any) {
	if !cond {
		panic("assertion failed")
	}
}

// Equal panics when got and want are not equal. The adjacent same-type
// parameters are the sanctioned assert-package shape, not a TS-N07 finding.
func Equal[T comparable](got, want T) {
	if got != want {
		panic("assertion failed: values differ")
	}
}
