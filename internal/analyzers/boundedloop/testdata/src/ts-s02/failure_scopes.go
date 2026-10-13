// A closure made outside the loop that writes the counter still defeats it,
// even when the loop sits inside another literal.
package fixture

// literalRewindsOuterCounter rewinds a counter declared outside the literal
// the loop sits in.
func literalRewindsOuterCounter(n int, run func(func())) {
	i := 0
	rewind := func() { i = 0 }
	run(func() {
		for ; i < n; i++ { // want `TS-S02: this counter does not bound the loop`
			rewind()
		}
	})
}
