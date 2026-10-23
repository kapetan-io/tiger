// Package seq hands out an iterator written in this module.
package seq

import "iter"

// Naturals yields every natural number and never ends.
func Naturals() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; yield(i); i++ {
		}
	}
}
