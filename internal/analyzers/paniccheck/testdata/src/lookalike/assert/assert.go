// A package that only calls itself assert is not tiger's assert package, so
// its panic is as naked as any other.
package assert

// Ok hand-rolls the crash path tiger's assert package owns.
func Ok(cond bool) {
	if !cond {
		panic("assertion failed") // want `TS-S18: panic is called directly here`
	}
}
