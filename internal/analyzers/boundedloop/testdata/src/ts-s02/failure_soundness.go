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

// registry holds a map that a loop measures.
type registry struct{ entries map[int]int }

// growsMapField inserts into the map field whose length is the limit.
func growsMapField(r *registry) {
	for i := 0; i < len(r.entries); i++ { // want `TS-S02: this counter does not bound the loop`
		r.entries[i+1000] = 1
	}
}

// index is a named map type with a method that inserts.
type index map[int]int

func (x index) add(key int) { x[key] = 1 }

// growsThroughMethod inserts into the map whose length is the limit by calling
// a method on it.
func growsThroughMethod(x index) {
	for i := 0; i < len(x); i++ { // want `TS-S02: this counter does not bound the loop`
		x.add(i + 1000)
	}
}

// fillsChannel sends on the channel whose length is the limit.
func fillsChannel(ch chan int) {
	for i := 0; i < len(ch); i++ { // want `TS-S02: this counter does not bound the loop`
		ch <- i
	}
}
