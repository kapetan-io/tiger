// Documented TS-F07 coverage gaps: writes the restricted points-to fragment
// cannot chase back to a parameter-rooted location, so they are silently
// absent from the computed frame instead of a finding.
package fixture

// fieldPointer returns r's handle pointer — an opaque hop the chase cannot
// see back through to a parameter-rooted location.
func fieldPointer(r *Recorder) *string {
	return r.handle
}

// setViaAlias writes r.handle only through h, a local bound to
// fieldPointer's return value.
//
// known-miss: ssalib.Writes chases a store's address through
// FieldAddr/IndexAddr/UnOp(MUL) chains rooted directly at a parameter or
// receiver; a pointer laundered through a function's return value is an
// opaque hop it cannot see through, so this write never joins the computed
// frame. A frame pinned on an exported caller of this helper would omit
// r.handle and still match exactly — the mismatch never surfaces as a
// finding, which is the known-miss boundary TS-P02 bounds, not a bug to
// fix here.
func (r *Recorder) setViaAlias(v string) {
	h := fieldPointer(r)
	*h = v
}
