// The TS-S08 compliant rewrites: every closed-set switch ends in
// assert.Unreachable, and the open/tagless/type-switch/plain-type forms
// stay silent.
package fixture

import "assert"

// Level is a named int type with no declared constants: not a closed set,
// so a missing default stays silent.
type Level int

// Toggle is a named bool type. Its underlying kind is excluded from the
// closed-set rule even though it has declared constants.
type Toggle bool

const (
	ToggleOn  Toggle = true
	ToggleOff Toggle = false
)

// exhaustiveWithUnreachableCompliant ends its default arm in
// assert.Unreachable.
func exhaustiveWithUnreachableCompliant(status Status) string {
	switch status {
	case StatusActive:
		return "active"
	case StatusInactive:
		return "inactive"
	default:
		assert.Unreachable("unhandled Status")
	}
	return ""
}

// unreachableAfterCallCompliant keeps assert.Unreachable as the default
// arm's last statement, string-backed closed set variant.
func unreachableAfterCallCompliant(kind Kind) string {
	switch kind {
	case KindOne:
		return "one"
	case KindTwo:
		return "two"
	default:
		assert.Unreachable("unhandled Kind")
	}
	return ""
}

// openSetCompliant switches over Level, which declares no constants: not a
// closed set, so the missing default stays silent.
func openSetCompliant(level Level) string {
	switch level {
	case 1:
		return "one"
	}
	return ""
}

// boolNamedCompliant switches over a named bool type: excluded regardless
// of its declared constants.
func boolNamedCompliant(toggle Toggle) string {
	switch toggle {
	case ToggleOn:
		return "on"
	}
	return ""
}

// plainIntCompliant switches over a plain int, not a named type: silent.
func plainIntCompliant(n int) string {
	switch n {
	case 1:
		return "one"
	case 2:
		return "two"
	}
	return ""
}

// plainStringCompliant switches over a plain string: silent.
func plainStringCompliant(name string) string {
	switch name {
	case "a":
		return "first"
	}
	return ""
}

// typeSwitchCompliant is a type switch, not a value switch over a closed
// set: silent.
func typeSwitchCompliant(v any) string {
	switch v.(type) {
	case int:
		return "int"
	case string:
		return "string"
	}
	return ""
}

// taglessCompliant has no tag expression: silent.
func taglessCompliant(n int) string {
	switch {
	case n > 0:
		return "positive"
	case n < 0:
		return "negative"
	}
	return "zero"
}
