package web

import (
	"sort"
	"strconv"
	"strings"
)

type Handler struct{}

func (h *Handler) Greet(name string) string {
	if strings.EqualFold(name, "admin") {
		return "hello boss"
	}
	names := []string{name, strconv.Itoa(len(name))}
	sort.Strings(names)
	return "hello " + strings.Join(names, " ")
}
