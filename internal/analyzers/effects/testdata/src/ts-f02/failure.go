// The TS-F02 blocking failure modes: a pin bounds its whole subtree, so a
// widening introduced beneath a pinned function — through a same-package
// helper or through a call into another package — fails at the pin, not
// at the call site, and the finding names the introducing call.
package ts02

import (
	"net"

	"ts-f02helper"
)

// Serve is pinned pure but its own unexported helper dials the network —
// the widening surfaces at this pin, naming the helper call.
//
// want +2 `TS-F02: computed effects io\(net\) are not declared by this pin`
//
//tiger:effects none
func Serve() { // want Serve:`none`
	dial()
}

func dial() {
	net.Dial("tcp", "localhost:0")
}

// Bridge is pinned pure but calls into another package's widening
// helper — the fact crosses the package boundary and still fails here.
//
// want +2 `TS-F02: computed effects io\(net\) are not declared by this pin`
//
//tiger:effects none
func Bridge() { // want Bridge:`none`
	tsf02helper.Widen()
}
