package web

import "example.com/shop/billing"

type Handler struct {
	billing *billing.Billing
}

func (h *Handler) Pay() int {
	return h.billing.Charge(10)
}
