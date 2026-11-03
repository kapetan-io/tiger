// Counter loops that read as disturbed only to an analysis that looks in the
// wrong scope: a loop inside a function literal that declares its counter,
// and a limit read from a struct another module owns.
package fixture

import "example.com/bench"

// insideLiteral counts inside a function literal, which declares the counter. A
// literal that declares the target holds its own writes, so only a literal that
// does not declare it counts against it (see literalRewindsOuterCounter).
func insideLiteral(run func(func())) {
	run(func() {
		for i := 0; i < 5; i++ {
			_ = i
		}
	})
}

// benchmarkLoop reads the count from a struct the testing package owns and
// calls its methods in the body; none of them writes N. A struct declared
// outside the module is exempt from the call check, because its own code does
// not reach into the fields a caller reads.
func benchmarkLoop(b *bench.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
	}
}
