// The TS-F07-facts reported findings: an unpinned exported function's
// computed frame, printed in pin syntax so ENG-151 can freeze it into a
// //tiger:frame pin by pasting it.
package fixture

// TouchLog writes r.log and carries no pin, so its computed frame is
// reported.
func (r *Recorder) TouchLog(msg string) { // want `TS-F07: TouchLog writes r\.log through its receiver or parameters — nothing to fix; to make tiger fail the build if that changes, add //tiger:frame r\.log` TouchLog:`0\.log`
	r.log = msg
}

// Inspect never writes through its receiver, so its computed frame is
// empty.
func (r *Recorder) Inspect() string { // want `TS-F07: Inspect writes nothing through its receiver or parameters — nothing to fix; to make tiger fail the build if that changes, add //tiger:frame none` Inspect:`^$`
	return r.log
}
