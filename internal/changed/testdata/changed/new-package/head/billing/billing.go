package billing

import (
	"example.com/fakedb"
	"example.com/shop/store"
)

type Ledger struct {
	pool  *fakedb.Pool
	users *store.Users
}

func (l *Ledger) Charge(id string) {
	l.users.Find(id)
	l.write(id)
	l.flush()
}

func (l *Ledger) write(id string) {
	_ = l.pool.Exec(id)
}

func (l *Ledger) flush() {
	record(l.pool)
}

func record(pool *fakedb.Pool) {
	_ = pool.Close()
}
