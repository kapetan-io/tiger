// Package pair declares an interface with two implementations outside
// _test.go files.
package pair

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

// OpenDisk returns a disk storage.
func OpenDisk() *diskStorage {
	return &diskStorage{written: 0}
}

// OpenMemory returns a memory storage.
func OpenMemory() *memStorage {
	return &memStorage{buffer: nil}
}
