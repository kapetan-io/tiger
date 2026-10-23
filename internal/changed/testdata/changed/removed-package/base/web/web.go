package web

import "example.com/shop/billing"

type Handler struct {
	ledger *billing.Ledger
}

func (h *Handler) Greet(name string) string {
	h.ledger.Charge(name)
	return "hello " + name
}
