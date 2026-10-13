// Package bench is a stand-in for a testing package outside the module: a
// struct whose count field the framework sets.
package bench

// B runs a benchmark N times.
type B struct{ N int }

// StopTimer pauses the clock.
func (b *B) StopTimer() {}
