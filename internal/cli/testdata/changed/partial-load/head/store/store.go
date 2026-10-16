package store

import "example.com/fakedb"

type Users struct {
	pool *fakedb.Pool
}

func (u *Users) Find(id string) string {
	_, _ = u.pool.Query(id)
	return id
}
