// Package single declares an interface with exactly one implementation.
package single

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

// Open returns the only storage there is.
func Open() *diskStorage {
	return &diskStorage{written: 0}
}
