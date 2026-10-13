// Counters that read as bounded and are not: the limit reads the counter, the
// limit may sit at the edge of the counter's type, or a cap has no report.
package fixture

// limitReadsCounter compares the counter with itself plus five.
func limitReadsCounter() {
	for i := 0; i < i+5; i++ { // want `TS-S02: this counter does not bound the loop`
	}
}

// byteToParameter counts a byte up to a parameter that may be 255.
func byteToParameter(n uint8) {
	for i := uint8(0); i <= n; i++ { // want `TS-S02: this counter does not bound the loop`
	}
}

// unsignedByTwo counts an unsigned number down by two toward a parameter, and
// wraps past zero.
func unsignedByTwo(n, floor uint) {
	for i := n; i > floor; i -= 2 { // want `TS-S02: this counter does not bound the loop`
	}
}

// labeledCapIsSilent labels a cap with no report after it.
func labeledCapIsSilent(limit int, done func() bool) {
labeled:
	for i := 0; i < limit && !done(); i++ { // want `TS-S02: this counter cap ends the loop without saying so`
	}
	goto labeled
}

// grow inserts into a map it is handed.
func grow(entries map[int]int, key int) { entries[key] = 1 }

// growsThroughHelper inserts into the map whose length is the limit by
// handing it to a function.
func growsThroughHelper(entries map[int]int) {
	for i := 0; i < len(entries); i++ { // want `TS-S02: this counter does not bound the loop`
		grow(entries, i+1000)
	}
}
