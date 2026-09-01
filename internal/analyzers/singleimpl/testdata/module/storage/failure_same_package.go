// Package storage declares a single implementation in the same package as
// its interface — the plain TS-X01 case.
package storage

// Storage is implemented once in this package, so TS-X01 fires here.
type Storage interface { // want `interface Storage has one implementation, diskStorage`
	Get(key string) (string, bool)
	Put(key, value string)
}

// diskStorage is Storage's only implementation.
type diskStorage struct {
	data map[string]string
}

// Get reads a key.
func (d *diskStorage) Get(key string) (string, bool) {
	v, ok := d.data[key]
	return v, ok
}

// Put writes a key.
func (d *diskStorage) Put(key, value string) {
	d.data[key] = value
}
