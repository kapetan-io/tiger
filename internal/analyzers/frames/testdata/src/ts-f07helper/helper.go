// Package helper is the ts-f07 corpus's dependency package: its exported
// writer's frame crosses the package boundary as a FrameFact, so the pinned
// callers in ts-f07 are checked against a frame they never computed locally.
package helper

// Ledger is the shared state the cross-package cases write.
type Ledger struct {
	Entries []string
}

// Scrub rewrites the ledger's entries in place.
func Scrub(l *Ledger) {
	l.Entries = nil
}
