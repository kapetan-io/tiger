// Package generated mixes a machine-written file with a hand-written one.
package generated

// Apply crashes on a malformed batch without going through assert.
func Apply(size int) {
	if size < 0 {
		panic("negative size")
	}
}
