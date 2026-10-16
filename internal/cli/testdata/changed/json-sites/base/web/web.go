package web

import "example.com/fakedb"

func Save(pool *fakedb.Pool) {
	_, _ = pool.Query("a")
	_ = pool.Close()
	_ = pool.Close()
}
