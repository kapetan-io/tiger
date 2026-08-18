// Package tsf02helper is the ts-f02 corpus's cross-package dependency: it
// exports one unpinned function with an effect, so ts-f02's pinned callers
// can prove TS-F02 enforcement crosses a package boundary through the
// go/analysis facts mechanism, not just same-package composition.
package tsf02helper

import "net"

// Widen dials the network — the effect a pinned caller must not silently
// inherit.
func Widen() {
	net.Dial("tcp", "localhost:0")
}
