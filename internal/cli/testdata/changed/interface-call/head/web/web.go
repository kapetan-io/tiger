package web

import "example.com/shop/store"

type Handler struct {
	users store.Users
}

func (h *Handler) Greet(id string) string {
	name, err := h.users.Find(id)
	if err != nil {
		return "hello stranger"
	}
	return "hello " + name
}
