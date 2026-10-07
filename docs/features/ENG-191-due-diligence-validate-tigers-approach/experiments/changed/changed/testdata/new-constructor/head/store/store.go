package store

import "example.com/fakedb"

type Queue struct {
	pool *fakedb.Pool
}

func (q *Queue) Retry(id string) error {
	pool, err := fakedb.Open("/tmp/retry.db")
	if err != nil {
		return err
	}
	return pool.Exec("retry " + id)
}
