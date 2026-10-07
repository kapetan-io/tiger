package web

import "context"

type Handler struct{}

func (h *Handler) Greet(ctx context.Context, name string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}
	return "hello " + name
}
