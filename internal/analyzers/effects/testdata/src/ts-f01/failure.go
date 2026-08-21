// The TS-F01 blocking failure modes: a pin is an exact bidirectional
// contract. An own-instruction effect the pin does not declare fails, a
// pin declaring an effect the computed set does not have fails, and a pin
// on an unexported function fails outright (invariant 3).
package ts01

// Grow is pinned pure but its body allocates a slice — an own-instruction
// widening the pin does not declare.
//
// want +2 `TS-F01: computed effects alloc are not declared by this pin`
//
//tiger:effects none
func Grow() []int { // want Grow:`none`
	return make([]int, 4)
}

// Quiet is pinned to read from disk but its body touches nothing — a
// superset pin that means nothing.
//
// want +2 `TS-F01: pin declares io\(disk\) that the computed effects do not have`
//
//tiger:effects io(disk)
func Quiet() int { // want Quiet:`io\(disk\)`
	return 1
}

// hidden is unexported; TS-F01 pins may only appear on an exported
// function or method (invariant 3) — the exported pins above already
// constrain whatever helper needs this one, per TS-F02's modularity.
//
// want +2 `TS-F01: a pin may only appear on an exported function or method`
//
//tiger:effects none
func hidden() {}
