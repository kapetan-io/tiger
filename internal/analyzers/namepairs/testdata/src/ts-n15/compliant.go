// The TS-N15 compliant rewrites: every pair uses its approved half so
// related names line up — and English prepositions inside longer names are
// not pair halves at all.
package fixture

// source replaces the banned single-token src.
var source string

// copyResource replaces srcPath and dstPath with the approved halves,
// which line up exactly because the table is built for equal length.
func copyResource(sourcePath string, targetPath string) {
}

// Conduit carries the input/output pair with the approved halves.
type Conduit struct {
	// input replaces the banned in.
	input int
	// output replaces the banned out.
	output int
}

// endsInUnreachable uses "In" as an English preposition inside a longer
// name; flagging it was a false positive this repository's own dogfood run
// surfaced, fixed in the analyzer per correctness constraint 7.
func endsInUnreachable() bool {
	return true
}

// ResolvesInRegistry is the exported form of the same preposition shape.
func ResolvesInRegistry() bool {
	return true
}
