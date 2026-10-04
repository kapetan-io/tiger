package store

import "example.com/fakedb"

type Queue struct {
	pool *fakedb.Pool
}

func (q *Queue) Retry(id string) error {
	if _, err := q.pool.Query(id); err != nil {
		return err
	}
	return q.pool.Exec("retry " + id)
}
