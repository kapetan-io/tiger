package web

import "example.com/fakedb"

type Handler struct {
	pool *fakedb.Pool
}

func (h *Handler) Greet(name string) string {
	record(h.pool, name)
	return "hello " + name
}

func record(pool *fakedb.Pool, name string) {
	_ = pool.Exec("insert " + name)
}
