// The TS-E02 failure modes: an error-typed result discarded through _ with
// no comment justifying the discard. Each case's blank discard sits on the
// statement's opening line, and the call continues onto a later line, so
// the `want` annotation itself is not mistaken for a trailing
// justification on the statement's closing line.
package fixture

import "os"

// closeQuiet discards the close error without saying why it is safe.
func closeQuiet(f *os.File) {
	_ = f. // want `TS-E02: discarding this error silently hides a failure`
		Close()
}

// lookupQuiet discards the error half of a paired result from one call.
func lookupQuiet() int {
	v, _ := lookupOne( // want `TS-E02: discarding this error silently hides a failure`
		1,
	)
	return v
}

func lookupOne(id int) (int, error) {
	return id, nil
}

// mixedDiscard discards the error half of two independent, single-valued
// right-hand-side expressions.
func mixedDiscard() int {
	total, _ := sumOne(), lookupErr( // want `TS-E02: discarding this error silently hides a failure`
		1,
	)
	return total
}

func sumOne() int {
	return 1
}

func lookupErr(id int) error {
	return nil
}
