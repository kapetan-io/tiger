// The TS-N13 failure modes: two-or-more-token identifiers whose last token
// restates a type instead of naming a role.
package fixture

// userStr restates the string type it holds.
func userStr() string { // want `TS-N13: "userStr" ends with the type-echo token "Str"`
	return ""
}

// Config carries several type-echoing fields.
type Config struct {
	// cfgStruct restates struct.
	cfgStruct int // want `TS-N13: "cfgStruct" ends with the type-echo token "Struct"`
	// idsSlice restates slice.
	idsSlice []int // want `TS-N13: "idsSlice" ends with the type-echo token "Slice"`
	// errList restates list.
	errList []error // want `TS-N13: "errList" ends with the type-echo token "List"`
}

// nodeArr and counterInt each restate their own type.
func countNodes(nodeArr []int, counterInt int) { // want `TS-N13: "nodeArr" ends with the type-echo token "Arr"` `TS-N13: "counterInt" ends with the type-echo token "Int"`
}

// routeTable restates map.
func routeTable() {
	var pathMap map[string]int // want `TS-N13: "pathMap" ends with the type-echo token "Map"`
	_ = pathMap
}

// customerPtr restates pointer.
func customerPtr() *int { // want `TS-N13: "customerPtr" ends with the type-echo token "Ptr"`
	return nil
}

// serverInterface restates interface.
type serverInterface interface { // want `TS-N13: "serverInterface" ends with the type-echo token "Interface"`
	// runChan restates chan.
	runChan(input chan int) // want `TS-N13: "runChan" ends with the type-echo token "Chan"`
}
