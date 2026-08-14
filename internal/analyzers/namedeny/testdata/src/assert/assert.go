// Package assert is a corpus miniature of the spec-fixed assert API. It is
// full of names that would otherwise trip TS-N12 and TS-N13, proving the
// package-name exemption actually silences them.
package assert

// Data is a banned-token field name that must stay silent because this
// package is named assert.
type Data struct {
	// valueStr is both a banned token and a type echo.
	valueStr string
}

// Ok panics when cond is false, taking a banned-token parameter name.
func Ok(cond bool, info string) {
	if !cond {
		panic(info)
	}
}
