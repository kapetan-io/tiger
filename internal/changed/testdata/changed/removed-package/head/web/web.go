package web

type Handler struct{}

func (h *Handler) Greet(name string) string {
	return "hello " + name
}
