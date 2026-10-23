package web

type Handler struct {
	requests chan<- string
}

func (h *Handler) Greet(name string) string {
	h.requests <- "insert " + name
	return "hello " + name
}
