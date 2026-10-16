package web

import "example.com/fakedb"

// Sweep replaces Cleanup but calls a different function, so it is no rename.
func Sweep() error {
	return fakedb.Ping(1)
}
