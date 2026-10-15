// Package poolreuse shows how to put a value back in a sync.Pool without
// leaking the last user's data to the next user.
package poolreuse

import "sync"

// Buffer is a pooled value that remembers who used it last.
type Buffer struct {
	User string
	Data []byte
}

var pool = sync.Pool{New: func() any { return new(Buffer) }}

// UseThenZero writes the buffer, then zeroes the whole value as the last write
// before Put. It returns the buffer it put back, because sync.Pool does not
// promise to hand the same one out again and a test needs to inspect it.
func UseThenZero(user string, data []byte) *Buffer {
	buffer, ok := pool.Get().(*Buffer)
	if !ok {
		buffer = new(Buffer)
	}
	buffer.User = user
	buffer.Data = append(buffer.Data[:0], data...)
	*buffer = Buffer{}
	pool.Put(buffer)
	return buffer
}

// ZeroThenUse is the broken shape: it zeroes the buffer when it takes it out,
// then writes it, so the zeroing precedes Put and TS-M05 accepts it, but the
// next Get sees this user's data.
func ZeroThenUse(user string, data []byte) *Buffer {
	buffer, ok := pool.Get().(*Buffer)
	if !ok {
		buffer = new(Buffer)
	}
	*buffer = Buffer{}
	buffer.User = user
	buffer.Data = append(buffer.Data[:0], data...)
	pool.Put(buffer)
	return buffer
}
