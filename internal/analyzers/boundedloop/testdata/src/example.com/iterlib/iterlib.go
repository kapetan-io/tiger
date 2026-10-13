// Package iterlib is a stand-in for a module outside the standard library
// that hands out iterators.
package iterlib

import "iter"

// Naturals yields every natural number and never ends.
func Naturals() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; yield(i); i++ {
		}
	}
}
