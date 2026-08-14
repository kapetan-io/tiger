// The compliant TS-C12 shape: every channel-carrying type declaration in
// this package lives here, in the file the analyzer picks as canonical
// (the lexicographically smallest name among files that declare one).
package fixture

// Event is what flows through Events.
type Event struct {
	ID int
}

// Events is the channel type through which workers exchange Event values.
type Events chan Event

// Job pairs unrelated state with a channel field; the struct still counts
// as a channel-carrying declaration because its type expression contains a
// ChanType.
type Job struct {
	done chan struct{}
}
