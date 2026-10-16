package store

import "example.com/fakedb"

type Writer struct {
	pool     *fakedb.Pool
	Requests chan string
	count    int
}

func (w *Writer) Run() {
	for range w.Requests {
		w.count++
	}
}
