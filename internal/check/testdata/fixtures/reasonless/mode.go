// Package reasonless marks an open type without saying why it is open.
package reasonless

// A bare escape does not attach, so the default arm below is still a finding.

//tiger:openenum
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
