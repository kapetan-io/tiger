// The TS-M05 failure modes: a zeroing that only covers one branch of a path
// to Put, a Reset method call standing in for the zeroing, a zeroing placed
// after the Put it was supposed to protect, and a value put by value.
package fixture

import "sync"

// Buffer is a pool element. Its Reset method is the shape TS-M05 no longer
// trusts, since nothing stops a Reset from being empty.
type Buffer struct {
	data []byte
}

// Reset clears data so a released Buffer never carries a previous caller's
// bytes into the next Get.
func (b *Buffer) Reset() {
	b.data = b.data[:0]
}

// Latch has a Reset that clears nothing.
type Latch struct {
	user string
}

// Reset is empty, and a rule that trusted the name would pass it.
func (l *Latch) Reset() {}

var bufferPool = sync.Pool{
	New: func() any { return new(Buffer) },
}

var latchPool = sync.Pool{
	New: func() any { return new(Latch) },
}

var countPool = sync.Pool{
	New: func() any { return new(int) },
}

// putAfterPartialZero zeroes buf only when cond is true, so the path where
// cond is false reaches Put with buf's previous contents still resident.
func putAfterPartialZero(cond bool, buf *Buffer) {
	if cond {
		*buf = Buffer{}
	}
	bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// putAfterResetCall calls Reset, which is not a zeroing: a Reset method is
// code the rule cannot see into.
func putAfterResetCall(buf *Buffer) {
	buf.Reset()
	bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// getEmptyResetAndPutBack is the call 7 fake: an empty Reset passes any check
// that trusts the method name, and the next Get sees the last user.
func getEmptyResetAndPutBack(user string) {
	latch, ok := latchPool.Get().(*Latch)
	if !ok {
		return
	}
	latch.user = user
	latch.Reset()
	latchPool.Put(latch) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// putBeforeZero zeroes buf only after the Put, which is too late to protect
// the caller who receives buf from the next Get.
func putBeforeZero(buf *Buffer) {
	bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
	*buf = Buffer{}
}

// putByValue puts an int pointer's value back, which cannot be zeroed in
// place.
func putByValue() {
	countPool.Put(7) // want `TS-M05: int is put into a sync\.Pool by value`
}

// getUseAndPutBack takes a Buffer straight from the pool, dirties it, and
// puts it back with no reset anywhere — the rule's primary failure mode.
// A Get-obtained value is never known-clean, so the no-reset-anywhere
// silence that protects parameter-passed values (the interprocedural
// known-miss) does not apply here.
func getUseAndPutBack(entry byte) {
	buf, ok := bufferPool.Get().(*Buffer)
	if !ok {
		return
	}
	buf.data = append(buf.data, entry)
	bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
}

// peekPooled takes a buffer from the pool and puts it back in a defer with no
// zeroing, so the next Get sees what the last user left.
func peekPooled() int {
	buf, ok := bufferPool.Get().(*Buffer)
	if !ok {
		return 0
	}
	defer bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
	return len(buf.data)
}
