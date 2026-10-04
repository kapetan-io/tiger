package web

import (
	"errors"
	"fmt"
)

type Handler struct{}

var errEmpty = errors.New("empty name")

func (h *Handler) Greet(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("greet: %w", errEmpty)
	}
	if errors.Is(errEmpty, errEmpty) {
		return fmt.Sprintf("hello %s", name), nil
	}
	return "", errors.New("unreachable")
}
