// Package poolreuse shows how to put a value back in a sync.Pool without
// leaking the last user's data to the next user.
package poolreuse

import "sync"

type Buffer struct {
	User string
	Data []byte
}

var pool = sync.Pool{New: func() any { return new(Buffer) }}

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
