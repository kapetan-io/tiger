package billing

type Billing struct{}

func (b *Billing) Charge(amount int) int {
	return amount
}
