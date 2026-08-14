// The TS-N13 compliant rewrites: names describe the role, not the type, and
// a single-token name (like a String method) stays silent.
package fixture

// userName replaces the type-echoing userStr.
func userName() string {
	return ""
}

// Settings replaces the type-echoing fields with role-based names.
type Settings struct {
	// port replaces cfgStruct.
	port int
	// ids replaces idsSlice — the plural already carries the "many" meaning.
	ids []int
	// causes replaces errList.
	causes []error
}

// nodes and counter replace the type-echoing parameter names.
func tallyNodes(nodes []int, counter int) {
}

// pathTable replaces the type-echoing local var pathMap.
func pathTable() {
	var paths map[string]int
	_ = paths
}

// customer replaces the type-echoing customerPtr.
func customer() *int {
	return nil
}

// server replaces the type-echoing serverInterface, and run replaces
// runChan.
type server interface {
	run(input chan int)
}

// duration is a single-token name; the two-token requirement means a plain
// type name like this never fires even though "duration" is a role, not a
// type-echo suffix.
type duration int

// stringer shows a single-token method name (String) staying silent — it
// ends in a type-echo-shaped word but has no second token to echo against.
type stringer struct{}

func (stringer) String() string {
	return ""
}
