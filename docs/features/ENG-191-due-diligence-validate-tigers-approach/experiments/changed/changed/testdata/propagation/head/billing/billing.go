package billing

import "example.com/fakedb"

type Billing struct {
	pool *fakedb.Pool
}

func (b *Billing) Charge(amount int) int {
	_ = b.pool.Exec("charge")
	return amount
}
