package annotations

// WaitReady puts the annotation on a loop that is not cursor-shaped (the
// condition is not a method call on a plain variable), so the waiver does
// not apply.
func WaitReady(ready func() bool) int {
	spins := 0
	//tiger:batched waits for the store to report ready
	for !ready() {
		spins++
	}
	return spins
}
