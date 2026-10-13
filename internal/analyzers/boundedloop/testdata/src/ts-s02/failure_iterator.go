// A range over an iterator is bounded only when the standard library wrote
// it; any other iterator may never stop.
package fixture

import (
	"iter"

	"example.com/iterlib"
)

// rangeOutside ranges over an iterator from a module tiger can't see into.
func rangeOutside() {
	for value := range iterlib.Naturals() { // want `TS-S02: this loop ranges over an iterator tiger can't see into`
		_ = value
	}
}

// rangeLocal ranges over an iterator built in this package.
func rangeLocal(seq iter.Seq[int]) {
	for value := range seq { // want `TS-S02: this loop ranges over an iterator tiger can't see into`
		_ = value
	}
}
