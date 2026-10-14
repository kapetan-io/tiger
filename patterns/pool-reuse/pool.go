// Package poolreuse shows how to put a value back in a sync.Pool without
// leaking the last user's data to the next user.
package poolreuse

import "sync"

// Buffer is a pooled value that remembers who used it last.
type Buffer struct {
	User string
	Data []byte
}

// Reset zeroes the whole value, so a field added later is cleared without
// editing this method.
func (b *Buffer) Reset() { *b = Buffer{} }

// LeakyBuffer has the same fields as Buffer.
type LeakyBuffer struct {
	User string
	Data []byte
}

// Reset clears nothing. TS-M05 accepts it because a Reset runs before Put, but
// it cannot see that the body is empty.
func (b *LeakyBuffer) Reset() {}

var (
	pool      = sync.Pool{New: func() any { return new(Buffer) }}
	leakyPool = sync.Pool{New: func() any { return new(LeakyBuffer) }}
)

// UseWithReset resets the buffer before Put. It returns the buffer it put
// back, because sync.Pool does not promise to hand the same one out again and
// a test needs to inspect it.
func UseWithReset(user string, data []byte) *Buffer {
	buffer, ok := pool.Get().(*Buffer)
	if !ok {
		buffer = new(Buffer)
	}
	buffer.User = user
	buffer.Data = append(buffer.Data[:0], data...)
	buffer.Reset()
	pool.Put(buffer)
	return buffer
}

// UseWithEmptyReset is the broken shape: Reset runs before Put and clears
// nothing, so the next Get sees this user's data.
func UseWithEmptyReset(user string, data []byte) *LeakyBuffer {
	buffer, ok := leakyPool.Get().(*LeakyBuffer)
	if !ok {
		buffer = new(LeakyBuffer)
	}
	buffer.User = user
	buffer.Data = append(buffer.Data[:0], data...)
	buffer.Reset()
	leakyPool.Put(buffer)
	return buffer
}
