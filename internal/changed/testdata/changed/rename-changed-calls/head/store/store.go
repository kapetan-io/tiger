package store

import "example.com/fakedb"

type Jobs struct {
	pool *fakedb.Pool
}

func (j *Jobs) Retry(id string) error {
	_ = fakedb.Ping(len(id))
	return j.pool.Exec("retry " + id)
}
