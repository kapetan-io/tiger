// The TS-F07 blocking failure modes: a write outside the pinned frame (both
// direct and reached through an unexported helper), a pin that declares a
// location the body never writes, and a frame pin on an unexported
// function.
package fixture

// WriteBoth is pinned to r.log only, but its body also writes
// r.checkpoint — a write outside the pinned frame.
//
// want +1 `TS-F07: computed frame writes r\.checkpoint, introduced at .*, outside the pinned frame`
//tiger:frame r.log
func (r *Recorder) WriteBoth(msg string) { // want WriteBoth:`0\.log`
	r.log = msg
	r.checkpoint = msg
}

// LogOnly is pinned to both r.log and r.checkpoint, but only ever writes
// r.log — a superset pin that declares a location it never touches.
//
// want +1 `TS-F07: pinned frame location r\.checkpoint is never written`
//tiger:frame r.log, r.checkpoint
func (r *Recorder) LogOnly(msg string) { // want LogOnly:`0\.checkpoint,0\.log`
	r.log = msg
}

// computeInternal is a plain unexported function; pinning it violates
// invariant 3 — a pin binds only exported functions and methods.
//
// want +1 `TS-F07: frame pin on unexported function computeInternal`
//tiger:frame r.log
func computeInternal(r *Recorder, msg string) {
	r.log = msg
}

// StoreVia is pinned to r.log only. Its body writes r.log directly and
// calls writeCheckpoint with the receiver passed straight through, which
// writes r.checkpoint — outside the pin, and the finding names the
// introducing call.
//
// want +1 `TS-F07: computed frame writes r\.checkpoint, introduced at .*, outside the pinned frame`
//tiger:frame r.log
func (r *Recorder) StoreVia(msg string) { // want StoreVia:`0\.log`
	r.log = msg
	writeCheckpoint(r, msg)
}

// writeCheckpoint is the unexported same-package helper StoreVia calls
// with its receiver passed straight through.
func writeCheckpoint(r *Recorder, msg string) {
	r.checkpoint = msg
}
