package web

import "example.com/fakedb"

type Handler struct {
	pool *fakedb.Pool
}

func (h *Handler) Greet(name string) string {
	return "hello " + name
}

func (h *Handler) Stop() {
	_ = h.pool.Close()
}
