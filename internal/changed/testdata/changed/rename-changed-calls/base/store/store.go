package store

import "example.com/fakedb"

type Queue struct {
	pool *fakedb.Pool
}

func (q *Queue) Retry(id string) error {
	return q.pool.Exec("retry " + id)
}
