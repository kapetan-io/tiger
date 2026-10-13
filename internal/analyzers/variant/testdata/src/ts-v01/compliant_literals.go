// Loops inside function literals are judged like loops anywhere else: a
// literal that declares a counter owns its writes.
package fixture

// countedInLiteral counts to a constant inside a function literal.
func countedInLiteral(run func(func())) {
	run(func() {
		for i := 0; i < 5; i++ {
			_ = i
		}
	})
}

// drainedInLiteral drains a slice the literal declares.
func drainedInLiteral(run func(func())) {
	run(func() {
		pending := []int{1, 2, 3}
		for len(pending) > 0 { // want `TS-V01: this loop in drainedInLiteral ends because len\(pending\) shrinks on every pass`
			pending = pending[1:]
		}
	})
}
