package web

import "io"

type Handler struct {
	out io.Writer
}

func (h *Handler) Greet(name string) string {
	_, _ = h.out.Write([]byte(name))
	return "hello " + name
}
