package web

import "example.com/shop/store"

type Handler struct {
	users store.Users
}

func (h *Handler) Greet(id string) string {
	return "hello " + id
}
