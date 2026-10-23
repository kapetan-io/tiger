package web

import "example.com/fakedb"

type Handler struct{}

// Spin sits in a package that reaches os but never calls it, and it recurses
// through Twirl, so only Ping prints.
func (h *Handler) Greet(name string) string {
	_ = fakedb.Spin(1)
	_ = fakedb.Ping(1)
	return "hello " + name
}
