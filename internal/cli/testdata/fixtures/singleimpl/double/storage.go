// Package double declares an interface whose second implementation is a
// test double in a _test.go file, which never counts.
package double

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
