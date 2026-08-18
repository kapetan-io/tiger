package fixture

// known-miss: ranging over a channel receives on every iteration with no
// shutdown case, but the receive never surfaces as a unary <-expression
// AST node, so selectctx cannot see it. boundedloop owns the shape of an
// unbounded loop, including this one.
func drain(values chan int) int {
	total := 0
	for v := range values {
		total += v
	}
	return total
}

// known-miss: doneResults is a plain data channel carrying real results,
// not a shutdown signal, but its name contains the "done" token, so the
// name-based fallback recognizes it as a shutdown channel. The bare
// receive silences a genuine blocking op with no termination path.
func receiveDoneResults(doneResults chan int) int {
	return <-doneResults
}

// known-miss: sem is a struct{}-typed semaphore token, not a shutdown
// signal, but the type-based recognition cannot tell a semaphore acquire
// from a shutdown handoff. The select case silences a genuine missing
// ctx.Done() finding.
func acquire(sem chan struct{}) {
	select {
	case <-sem:
	}
}
