package l14

// x // want `TS-L14`
func Plain() {}

// first line of a group // want `TS-L14`
// second line, same group
// third line, same group
func Group() {}

func Inline() {
	// inside a body // want `TS-L14`
}
