// The TS-S08 failure modes: a switch over a closed set that has no default
// arm, or whose default arm does not end in assert.Unreachable.
package fixture

// Status is a closed set: consts of this exact type are declared in this
// package.
type Status int

const (
	StatusActive Status = iota
	StatusInactive
)

// Kind is a second closed set, string-backed, used for the
// wrong-last-statement failure mode.
type Kind string

const (
	KindOne Kind = "one"
	KindTwo Kind = "two"
)

// missingDefault has no default arm at all.
func missingDefault(status Status) string {
	switch status { // want `TS-S08: switch over closed set Status has no default arm`
	case StatusActive:
		return "active"
	case StatusInactive:
		return "inactive"
	}
	return ""
}

// wrongDefaultStatement has a default arm whose last statement is a plain
// return, not a call to assert.Unreachable.
func wrongDefaultStatement(kind Kind) string {
	switch kind {
	case KindOne:
		return "one"
	case KindTwo:
		return "two"
	default: // want `TS-S08: switch over closed set Kind's default arm does not end in assert\.Unreachable`
		return "unknown"
	}
}

// emptyDefaultBody has a default arm with no statements at all.
func emptyDefaultBody(status Status) string {
	switch status {
	case StatusActive:
		return "active"
	default: // want `TS-S08: switch over closed set Status's default arm does not end in assert\.Unreachable`
	}
	return ""
}

// TransferMode is marked //tiger:openenum: the wire can send a mode this
// package doesn't name yet, so a default arm is still required — just not
// one ending in assert.Unreachable.
//
//tiger:openenum
type TransferMode string

const (
	TransferModePush TransferMode = "push"
	TransferModePull TransferMode = "pull"
)

// openEnumMissingDefault has no default arm at all: still a failure for a
// marked type, because the wire can still send an unnamed value.
func openEnumMissingDefault(mode TransferMode) string {
	switch mode { // want `TS-S08: switch over open enum TransferMode has no default arm`
	case TransferModePush:
		return "push"
	case TransferModePull:
		return "pull"
	}
	return ""
}
