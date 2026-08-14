// The TS-N08 compliant rewrite: a named type with a bool underlying type
// instead of a plain bool.
package fixture

// SyncMode is a named type over bool so the call site reads.
type SyncMode bool

// SyncImmediate and SyncDeferred name the two states.
const (
	SyncImmediate SyncMode = true
	SyncDeferred  SyncMode = false
)

// saveCompliant persists state using the named SyncMode instead of a plain
// bool.
func saveCompliant(mode SyncMode) {
	_ = mode
}

// literalSaveCompliant assigns a func literal using the named SyncMode
// type.
var literalSaveCompliant = func(mode SyncMode) {
	_ = mode
}

// SaverCompliant is an interface whose method uses the named SyncMode type.
type SaverCompliant interface {
	// Save takes the named SyncMode type.
	Save(mode SyncMode)
}
