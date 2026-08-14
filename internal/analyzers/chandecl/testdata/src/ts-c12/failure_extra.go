// failure_extra.go declares channel-carrying types outside compliant.go —
// the TS-C12 failure mode. compliant.go sorts first, so it is the
// canonical file and every declaration here must move there.
package fixture

// Signals is declared outside the package's canonical channel-type file.
type Signals chan int // want `TS-C12: channel type declared outside compliant\.go`

// Pipe pairs unrelated state with a channel field; it still counts.
type Pipe struct { // want `TS-C12: channel type declared outside compliant\.go`
	acks chan int
}
