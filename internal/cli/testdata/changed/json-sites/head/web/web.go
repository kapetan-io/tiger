package web

import "example.com/fakedb"

func Save(pool *fakedb.Pool) {
	_, _ = pool.Query("a")
	_ = pool.Exec("b")
	if pool != nil {
		_ = pool.Exec("c")
	}
	_ = pool.Exec("d")
}
