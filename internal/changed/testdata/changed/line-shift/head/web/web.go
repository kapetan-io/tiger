package web

import "example.com/fakedb"

type Handler struct {
	pool *fakedb.Pool
}

// Greet gained a comment and a blank line, so its call moved down two lines.

func (h *Handler) Greet(name string) string {
	_ = h.pool.Exec("insert " + name)
	return "hello " + name
}
