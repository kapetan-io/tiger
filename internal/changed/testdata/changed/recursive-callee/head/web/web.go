package web

import "example.com/fakedb"

type Handler struct{}

func (h *Handler) Greet(name string) string {
	_ = fakedb.Ping(1)
	_ = fakedb.Pong(1)
	return "hello " + name
}
