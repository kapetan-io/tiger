// known-miss: the in/out halves match whole identifiers only, because as
// camelCase tokens they are usually English prepositions (endsInUnreachable).
// The cost is that abbreviation-prefixed names like inBuf and outValue are
// not caught; renaming those is left to review until a token-position
// heuristic earns its way in.
package fixture

// inBuf uses "in" as an abbreviation prefix; the analyzer deliberately
// stays silent (whole-identifier matching only).
var inBuf []byte

// assembleReply names its result with an "out" prefix; also deliberately
// silent for the same reason.
func assembleReply() (outValue int) {
	return 0
}
