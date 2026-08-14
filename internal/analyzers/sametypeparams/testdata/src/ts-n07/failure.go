// The TS-N07 failure modes: adjacent parameters sharing a type, in both the
// grouped-name and separate-field forms, and a parameter list over four.
package fixture

// copySameField takes two adjacent int parameters declared in one field.
func copySameField(retries, timeout int) { // want `TS-N07: adjacent parameters share type int`
	_, _ = retries, timeout
}

// copySeparateFields takes two adjacent int parameters declared as separate
// fields, with a differently typed parameter ahead of them.
func copySeparateFields(source string, retries int, timeout int) { // want `TS-N07: adjacent parameters share type int`
	_, _, _ = source, retries, timeout
}

// tooManyParams has more than four parameters, all of different types so
// only the count fires.
func tooManyParams(a int, b string, c float64, d byte, e rune) { // want `TS-N07: 5 parameters`
	_, _, _, _, _ = a, b, c, d, e
}

// Copier is an interface whose method has the same failure mode.
type Copier interface {
	// Copy takes two adjacent int parameters.
	Copy(retries, timeout int) // want `TS-N07: adjacent parameters share type int`
}
