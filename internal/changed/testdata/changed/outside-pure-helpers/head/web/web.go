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
	var out strings.Builder
	out.WriteString("hello ")
	names := []string{name, strconv.Itoa(len(name))}
	sort.Strings(names)
	out.WriteString(strings.Join(names, " "))
	return out.String()
}
