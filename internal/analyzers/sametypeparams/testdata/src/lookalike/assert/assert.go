// A package that only calls itself assert is not tiger's assert package, so
// its signatures get no exemption.
package assert

// Equal takes adjacent parameters of one type.
func Equal(got, want int) { // want `TS-N07: adjacent parameters share type int`
	_, _ = got, want
}
