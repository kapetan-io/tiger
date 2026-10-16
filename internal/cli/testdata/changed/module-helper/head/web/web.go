package web

import "example.com/shop/ids"

type Handler struct{}

func (h *Handler) Greet(name string) string {
	return "hello " + ids.Clean(name)
}
