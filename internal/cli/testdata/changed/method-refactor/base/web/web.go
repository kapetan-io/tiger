package web

import "example.com/fakedb"

type Handler struct {
	pool *fakedb.Pool
}

func (h *Handler) Greet(name string) string {
	_ = h.pool.Exec("insert " + name)
	return "hello " + name
}
