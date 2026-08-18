// The TS-F07 compliant shapes: a pin that matches the computed frame
// exactly, empty or not, whether the write happens directly in the pinned
// function's own body or is reached through an unexported same-package
// helper.
package fixture

// Recorder is the shared receiver type for the ts-f07 corpus: two
// independently trackable parameter-rooted locations, plus a pointer field
// (handle) that knownmiss.go uses to demonstrate the chase's alias
// boundary.
type Recorder struct {
	log        string
	checkpoint string
	handle     *string
}

// Pure never writes through any parameter, so the empty frame is exact.
//
//tiger:frame none
func Pure() {} // want Pure:`^$`

// Direct writes r.log directly; its pin is exact.
//
//tiger:frame r.log
func (r *Recorder) Direct(msg string) { // want Direct:`0\.log`
	r.log = msg
}

// ViaHelper reaches r.log only through setLog, an unexported same-package
// helper the receiver is passed straight through to; the rebased write
// still lands inside the pin.
//
//tiger:frame r.log
func (r *Recorder) ViaHelper(msg string) { // want ViaHelper:`0\.log`
	setLog(r, msg)
}

// setLog is the unexported helper ViaHelper calls with its receiver passed
// straight through.
func setLog(r *Recorder, msg string) {
	r.log = msg
}
