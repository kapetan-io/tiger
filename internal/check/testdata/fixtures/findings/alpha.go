// Package findings holds deterministic blocking findings across two files.
package findings


func Apply(size int) {
	if size < 0 {
		panic("negative size")
	}
}
