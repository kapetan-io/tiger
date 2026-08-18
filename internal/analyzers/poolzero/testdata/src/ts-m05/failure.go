// The TS-M05 failure modes: a reset that only covers one branch of a path
// to Put, a pooled type with no Reset method at all, and a reset placed
// after the Put it was supposed to protect.
package fixture

import "sync"

// Buffer is a pool element with a Reset method — the compliant shape.
type Buffer struct {
	data []byte
}

// Reset clears data so a released Buffer never carries a previous caller's
// bytes into the next Get.
func (b *Buffer) Reset() {
	b.data = b.data[:0]
}

var bufferPool = sync.Pool{
	New: func() any { return new(Buffer) },
}

// Widget is a pool element with no Reset method at all.
type Widget struct {
	data []byte
}

var widgetPool = sync.Pool{
	New: func() any { return new(Widget) },
}

// putAfterPartialReset resets buf only when cond is true, so the path where
// cond is false reaches Put with buf's previous contents still resident.
func putAfterPartialReset(cond bool, buf *Buffer) {
	if cond {
		buf.Reset()
	}
	bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by a reset of the same value on every path`
}

// putNoResetMethod puts a Widget, which has no Reset method, so there is no
// single checkable call that zeroes it on release.
func putNoResetMethod(w *Widget) {
	widgetPool.Put(w) // want `TS-M05: .*Widget is put into a sync\.Pool but has no Reset method`
}

// putBeforeReset resets buf only after the Put, which is too late to
// protect the caller who receives buf from the next Get.
func putBeforeReset(buf *Buffer) {
	bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by a reset of the same value on every path`
	buf.Reset()
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
	bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by a reset of the same value on every path`
}
