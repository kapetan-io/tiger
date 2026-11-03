// Package openenum declares a type whose set of values is open, with the
// reason tiger needs to count the claim.
package openenum



//tiger:openenum peers on the wire add modes before we upgrade
type Mode int

const (
	ModeCopy Mode = iota
	ModeMove
)


func Name(mode Mode) string {
	switch mode {
	case ModeCopy:
		return "copy"
	case ModeMove:
		return "move"
	default:
		return "unknown"
	}
}
