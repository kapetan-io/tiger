// Documented TS-M05 coverage gaps: the analysis is intraprocedural and
// identity-exact, so a reset that lives in a different function than its
// Put, or reaches Put only through an alias, is invisible to it.
package fixture

// releaseHelperKnownMiss puts buf back into the pool; it never resets buf
// itself, because that is the caller's job.
//
// known-miss: releaseCallerKnownMiss resets buf before calling this helper,
// but that reset lives in a different function's SSA. A function that puts
// a value it never resets locally is indistinguishable, from inside this
// function alone, from a caller that already reset it — so this analyzer
// stays silent rather than guess, and fires only when a reset of the exact
// value is visible somewhere in the same function.
func releaseHelperKnownMiss(buf *Buffer) {
	bufferPool.Put(buf)
}

// releaseCallerKnownMiss resets buf and hands it to a helper that puts it —
// safe in practice, silent here for the same reason: this analyzer never
// looks past a function's own boundary.
func releaseCallerKnownMiss(buf *Buffer) {
	buf.Reset()
	releaseHelperKnownMiss(buf)
}

// aliasKnownMiss resets buf through one SSA-level load of a slice element
// and puts it through a second, separate load of the same element. Both
// loads read the same pointer at run time, but the identity-exact check
// only recognizes the literal same ssa.Value, so the reset does not count
// for the Put.
//
// known-miss: reset(a) does not make Put(b) provably reset even when a and
// b hold the same object, because this analyzer performs no alias
// analysis. The reverse direction — reset through the alias, Put through
// the original name — would be a real bleed this analyzer equally cannot
// see; it is not special-cased safe, it is simply invisible either way.
func aliasKnownMiss(buf *Buffer) {
	holders := []*Buffer{buf}
	holders[0].Reset()
	bufferPool.Put(holders[0])
}
