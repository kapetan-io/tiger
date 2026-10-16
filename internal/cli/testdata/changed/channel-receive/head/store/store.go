package store

import "example.com/fakedb"

type Writer struct {
	pool     *fakedb.Pool
	Requests chan string
	count    int
}

func (w *Writer) Run() {
	for query := range w.Requests {
		w.count++
		_ = w.pool.Exec(query)
	}
}
