package web

import "example.com/fakedb"

type Handler struct {
	pool *fakedb.Pool
}

func (h *Handler) Greet(name string) string {
	h.record(name)
	return "hello " + name
}

func (h *Handler) record(name string) {
	write := func(query string) { _ = h.pool.Exec(query) }
	write("insert " + name)
}
