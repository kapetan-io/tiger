package gaps

import "iter"

// naturals yields forever. Its own loop is checked where it is written, but a
// caller's range over it is treated as ending by syntax.
func naturals() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < len(naturalsForever); i++ {
			if !yield(i) {
				return
			}
			i--
		}
	}
}

// naturalsForever gives naturals a len to pass TS-S02 while never ending.
var naturalsForever = []int{0}

// RangeNaturals ranges over an iterator that never ends.
func RangeNaturals() {
	for value := range naturals() {
		_ = value
	}
}

// GotoLoop loops with goto, which neither loop rule inspects.
func GotoLoop() {
loop:
	goto loop
}
