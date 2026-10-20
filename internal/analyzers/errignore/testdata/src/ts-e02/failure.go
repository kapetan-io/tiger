// The TS-E02 failure modes: an error-typed result discarded through _,
// with or without a comment beside it.
package fixture

import "os"

func closeQuiet(f *os.File) {
	_ = f. // want `TS-E02: this error is discarded with _, so a failure here goes unnoticed`
		Close()
}

// lookupQuiet discards the error half of a paired result from one call.
func lookupQuiet() int {
	v, _ := lookupOne( // want `TS-E02: this error is discarded with _, so a failure here goes unnoticed`
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
	total, _ := sumOne(), lookupErr( // want `TS-E02: this error is discarded with _, so a failure here goes unnoticed`
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

// closeTrailing has a trailing comment, which does not clear the finding.
func closeTrailing(f *os.File) {
	_ = f.Close() // want `TS-E02: this error is discarded with _, so a failure here goes unnoticed`
}

// closeAbove has a comment on the line above, which does not clear it either.
func closeAbove(f *os.File) {
	// best effort cleanup; nothing actionable if this fails
	_ = f.Close() // want `TS-E02: this error is discarded with _, so a failure here goes unnoticed`
}

// lookupTrailing discards the error half of a paired result under a comment.
func lookupTrailing() int {
	v, _ := lookupOne(1) // want `TS-E02: this error is discarded with _, so a failure here goes unnoticed`
	return v
}
