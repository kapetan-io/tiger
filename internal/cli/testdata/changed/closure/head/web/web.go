package web

import "example.com/fakedb"

type Handler struct {
	pool *fakedb.Pool
}

func (h *Handler) Greet(name string) string {
	go func() {
		defer func() { _ = h.pool.Close() }()
		_ = h.pool.Exec("insert " + name)
	}()
	return "hello " + name
}
