// Package reasonless marks an open type without saying why it is open.
package reasonless

// Mode is the transfer mode a peer asks for.
//
//tiger:openenum
type Mode int

const (
	ModeCopy Mode = iota
	ModeMove
)

// Name labels a mode, with a catch-all for modes this build predates.
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
