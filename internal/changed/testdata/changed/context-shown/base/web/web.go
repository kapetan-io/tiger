package web

import "context"

type Handler struct{}

func (h *Handler) Greet(ctx context.Context, name string) string {
	return "hello " + name
}
