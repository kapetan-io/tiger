package store

import "example.com/fakedb"

type Users interface {
	Find(id string) (string, error)
}

type DBUsers struct {
	pool *fakedb.Pool
}

func (u *DBUsers) Find(id string) (string, error) {
	return u.pool.Query(id)
}
