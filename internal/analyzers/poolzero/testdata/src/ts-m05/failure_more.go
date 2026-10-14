// More TS-M05 failure modes: values that look zeroed but are not T{}, a
// zeroing of a different value, and pools reached through a field or closure.
package fixture

import "sync"

// Tally is a pool element with data a previous user leaves behind.
type Tally struct {
	items []int
	owner string
}

var tallyPool = sync.Pool{New: func() any { return new(Tally) }}

var slicePool = sync.Pool{New: func() any { return new([]byte) }}

// putAfterNonZeroCopy overwrites the Tally with a copy that is not the zero
// value, so the owner string survives into the next Get.
func putAfterNonZeroCopy(owner string) {
	tally, ok := tallyPool.Get().(*Tally)
	if !ok {
		return
	}
	fresh := Tally{owner: owner}
	*tally = fresh
	tallyPool.Put(tally) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// putAfterNonZeroLiteral overwrites the Tally with a literal that sets a
// field, which is not T{}.
func putAfterNonZeroLiteral(owner string) {
	tally, ok := tallyPool.Get().(*Tally)
	if !ok {
		return
	}
	*tally = Tally{owner: owner}
	tallyPool.Put(tally) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// putAfterTruncate shortens the slice, which keeps the bytes resident.
func putAfterTruncate(data []byte) {
	buf, ok := slicePool.Get().(*[]byte)
	if !ok {
		return
	}
	*buf = append((*buf)[:0], data...)
	slicePool.Put(buf) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// putAfterZeroingOther zeroes a different Tally than the one it puts.
func putAfterZeroingOther(other *Tally) {
	tally, ok := tallyPool.Get().(*Tally)
	if !ok {
		return
	}
	tally.owner = "x"
	*other = Tally{}
	tallyPool.Put(tally) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// Arena keeps its pool in a field.
type Arena struct {
	pool sync.Pool
}

// Recycle puts a Tally back without zeroing it.
func (a *Arena) Recycle(owner string) {
	tally, ok := a.pool.Get().(*Tally)
	if !ok {
		return
	}
	tally.owner = owner
	a.pool.Put(tally) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// recycleInClosure puts a Tally back from inside a function literal.
func recycleInClosure(owner string) func() {
	return func() {
		tally, ok := tallyPool.Get().(*Tally)
		if !ok {
			return
		}
		tally.owner = owner
		tallyPool.Put(tally) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
	}
}

// zeroInLoopPutAfter zeroes only inside a loop body that may not run.
func zeroInLoopPutAfter(rounds int) {
	tally, ok := tallyPool.Get().(*Tally)
	if !ok {
		return
	}
	for range rounds {
		*tally = Tally{}
	}
	tallyPool.Put(tally) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}
