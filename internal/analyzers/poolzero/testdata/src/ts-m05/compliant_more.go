// More TS-M05 compliant shapes: a zeroing that guards the only path to Put,
// and a zeroing before a deferred Put.
package fixture

// putAfterGuard returns early on one branch and zeroes before the only path
// that reaches Put.
func putAfterGuard(bad bool, tally *Tally) {
	if bad {
		return
	}
	*tally = Tally{}
	tallyPool.Put(tally)
}

// zeroThenDeferPut zeroes right after Get and defers the Put.
func zeroThenDeferPut() int {
	tally, ok := tallyPool.Get().(*Tally)
	if !ok {
		return 0
	}
	*tally = Tally{}
	defer tallyPool.Put(tally)
	return len(tally.items)
}

// Shelf keeps its pool in a field and zeroes before the Put.
type Shelf struct {
	pool Arena
}

// Return zeroes the Tally right after Get and puts it back.
func (s *Shelf) Return(owner string) {
	tally, ok := s.pool.pool.Get().(*Tally)
	if !ok {
		return
	}
	tally.owner = owner
	*tally = Tally{}
	s.pool.pool.Put(tally)
}
