// More iterators tiger can't see into.
package fixture

import "iter"

func naturalsHere() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < 10; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// rangeSamePackage ranges over an iterator this package declares.
func rangeSamePackage() {
	for value := range naturalsHere() { // want `TS-S02: this loop ranges over an iterator tiger can't see into`
		_ = value
	}
}
