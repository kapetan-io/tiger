package store

import "example.com/fakedb"

type Writer struct {
	pool     *fakedb.Pool
	Requests chan string
}

func (w *Writer) Run() {
	for query := range w.Requests {
		_ = w.pool.Exec(query)
	}
}
