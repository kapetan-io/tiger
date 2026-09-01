// Package local claims closed dispatch and keeps it: the one interface call
// has a receiver built from a single concrete value in the same function.
//
//tiger:restrict closed-dispatch
package local

// Storage persists bytes.
type Storage interface {
	Write(data []byte) int
}

type diskStorage struct {
	written int
}

// Write records the bytes.
func (d *diskStorage) Write(data []byte) int {
	d.written += len(data)
	return d.written
}

type memStorage struct {
	buffer []byte
}

// Write appends the bytes.
func (m *memStorage) Write(data []byte) int {
	m.buffer = append(m.buffer, data...)
	return len(m.buffer)
}

// Save writes through a storage built right here.
func Save(data []byte) int {
	var s Storage = &diskStorage{written: 0}
	return s.Write(data)
}
