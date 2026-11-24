package cgocycle

import "C"

// Zebra and Apple form one cycle across two cgo files. The finding belongs at
// Zebra, in the file that sorts first by name.
func Zebra(n int) int { // want `TS-S01: Apple → Zebra → Apple is a cycle of calls`
	return Apple(n) + int(C.int(0))
}

// variant 1

// variant 2

// variant 3
