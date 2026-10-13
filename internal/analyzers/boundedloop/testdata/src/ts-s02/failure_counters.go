// The TS-S02 counter failures: a counter is a bound only when it moves toward
// a limit that nothing changes, in a top-level && term.
package fixture

// orCounter puts the counter on one side of ||, so the other side alone keeps
// the loop going.
func orCounter(done func() bool) {
	for i := 0; i < 10 || !done(); i++ { // want `TS-S02: this counter does not bound the loop`
	}
}

// wrongDirection moves its counter away from the limit.
func wrongDirection(n int) {
	for i := 0; i < n; i-- { // want `TS-S02: this counter does not bound the loop`
	}
}

// counterUndone cancels its own counter in the body.
func counterUndone(n int) {
	for i := 0; i < n; i++ { // want `TS-S02: this counter does not bound the loop`
		i--
	}
}

// limitGrows raises a plain integer limit as fast as the counter climbs.
func limitGrows(n int) {
	for i := 0; i < n; i++ { // want `TS-S02: this counter does not bound the loop`
		n++
	}
}

// stepsPastLimit steps by 2 toward an odd limit with !=, so it never lands
// on it.
func stepsPastLimit() {
	for i := 0; i != 7; i += 2 { // want `TS-S02: this counter does not bound the loop`
	}
}

// variableStep moves by an amount nobody pins down, which may be zero.
func variableStep(n, step int) {
	for i := 0; i < n; i += step { // want `TS-S02: this counter does not bound the loop`
	}
}

// limitFromFunctionValue asks a function value for the limit on every pass.
func limitFromFunctionValue(limit func() int) {
	for i := 0; i < limit(); i++ { // want `TS-S02: this counter does not bound the loop`
	}
}

// counterThroughPointer lets a helper move the counter.
func counterThroughPointer(n int) {
	for i := 0; i < n; i++ { // want `TS-S02: this counter does not bound the loop`
		reset(&i)
	}
}

func reset(counter *int) { *counter = 0 }

// counterInClosure moves the counter from a closure made before the loop.
func counterInClosure(n int) {
	i := 0
	rewind := func() { i = 0 }
	for ; i < n; i++ { // want `TS-S02: this counter does not bound the loop`
		rewind()
	}
}
