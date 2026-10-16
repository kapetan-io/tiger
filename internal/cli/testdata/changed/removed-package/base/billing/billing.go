package billing

import "example.com/fakedb"

type Ledger struct {
	pool *fakedb.Pool
}

func (l *Ledger) Charge(id string) {
	_ = l.pool.Exec(id)
}

func Refund(pool *fakedb.Pool, id string) {
	_ = pool.Exec("refund " + id)
}
