// Package findings holds deterministic blocking findings across two files.
package findings

// Apply crashes on a malformed batch without going through assert.
func Apply(size int) {
	if size < 0 {
		panic("negative size")
	}
}
