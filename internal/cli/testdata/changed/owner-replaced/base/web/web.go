package web

import "example.com/fakedb"

func Cleanup(pool *fakedb.Pool) {
	_ = pool.Exec("cleanup")
}
