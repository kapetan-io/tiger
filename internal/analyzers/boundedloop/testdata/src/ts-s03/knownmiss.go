// Documented TS-S03 coverage gaps introduced by the second shutdown
// shape: both recognitions only relax a check, so a false match silences
// a real finding instead of creating one.
package fixture

// semaphoreSignalKnownMiss uses a struct{}-typed channel purely as a
// concurrency-limiting token, not a shutdown signal — but the type-based
// recognition treats every struct{}-element channel as a recognized
// shutdown shape by type alone, so this select's real TS-S03 finding
// (nothing here actually tells the loop to stop) goes silent.
//
// known-miss: distinguishing "signal used for shutdown" from "signal used
// as a semaphore" needs usage-site judgment a single-package syntactic
// check does not have; the type-based recognition is deliberately
// permissive (a false match only relaxes a finding, never adds one).
func semaphoreSignalKnownMiss(tokens chan struct{}, input <-chan int, output chan<- int) {
	for {
		select {
		case <-tokens:
			// a semaphore token freed up, not a shutdown signal
		case value := <-input:
			output <- value * 2
		}
	}
}

// doneCountsKnownMiss's channel just happens to have "done" in its name —
// it reports completed-item counts, it is not a shutdown signal — but the
// name-based fallback matches on substring alone, so this select's real
// TS-S03 finding goes silent too.
//
// known-miss: same permissive-by-design tradeoff as the struct{} case;
// closing it needs a naming convention this heuristic does not enforce.
func doneCountsKnownMiss(doneCounts chan int, input <-chan int, output chan<- int) {
	for {
		select {
		case n := <-doneCounts:
			output <- n
		case value := <-input:
			output <- value * 2
		}
	}
}
