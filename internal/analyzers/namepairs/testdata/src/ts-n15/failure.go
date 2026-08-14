// The TS-N15 failure modes: declared identifiers built from the wrong half
// of a committed name pair.
package fixture

// src is a single-token identifier equal to a banned half.
var src string // want `TS-N15: "src" uses the banned half`

// copySettings takes both banned halves of the same pair as parameters.
func copySettings(srcPath string, dstPath string) { // want `TS-N15: "srcPath" uses the banned half` `TS-N15: "dstPath" uses the banned half`
}

// Pipe carries the in/out pair as whole-identifier fields, which is the
// only form those halves match: as whole words they are abbreviations, not
// prepositions.
type Pipe struct {
	// in is the banned half of the in/input pair.
	in int // want `TS-N15: "in" uses the banned half`
	// out is the banned half of the out/output pair.
	out int // want `TS-N15: "out" uses the banned half`
}
