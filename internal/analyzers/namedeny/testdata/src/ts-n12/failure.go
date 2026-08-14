// The TS-N12 failure modes: declared identifiers built from the committed
// deny dictionary of semantically empty tokens.
package fixture

// Manager is a type name built entirely from a banned token.
type Manager struct { // want `TS-N12: "Manager" uses semantically empty token "manager"`
	// Data is a struct field named after a banned token.
	Data int // want `TS-N12: "Data" uses semantically empty token "data"`
}

// registerRoute ends with a banned token even though the rest of the name
// is fine.
func registerRouteHelper() { // want `TS-N12: "registerRouteHelper" uses semantically empty token "helper"`
}

// process is a package-level var named after a banned token.
var process string // want `TS-N12: "process" uses semantically empty token "process"`

// fetchRecord takes a banned-token parameter and returns a banned-token
// named result.
func fetchRecord(item string) (value int) { // want `TS-N12: "item" uses semantically empty token "item"` `TS-N12: "value" uses semantically empty token "value"`
	return 0
}

// collectRows shows the plural form of a dictionary token also matching, on
// a local variable rather than the function name.
func collectRows() {
	objects := []string{} // want `TS-N12: "objects" uses semantically empty token "objects"`
	_ = objects
}

// declareLocal shows a local var declared with the var keyword hitting the
// dictionary the same way a package-level one does.
func declareLocal() {
	var temp int // want `TS-N12: "temp" uses semantically empty token "temp"`
	_ = temp
}
