// Package helper is the smoke fixture's second package: it exports an
// unpinned function with a network effect, so a pinned caller in the smoke
// package can only be caught if the effects fact crossed the import. It
// declares no-reflect so it weakens nothing smoke claims.
//
//tiger:restrict no-reflect
package helper

import "net"

// Widen dials the network — the effect a pinned caller must not silently
// inherit.
func Widen() {
	_, _ = net.Dial("tcp", "localhost:0") // effect matters, result does not
}
