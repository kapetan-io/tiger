package service

import "example.com/fakedb"

type Service struct {
	pool *fakedb.Pool
}

// Handle and Other make the same calls as before. Only the functions they
// call changed, so only those functions print.
func (s *Service) Handle(id string) {
	stamp(id)
}

func (s *Service) Other(id string) {
	s.pool.Close()
	touch(id)
}

func stamp(id string) {
	_ = fakedb.Ping(len(id))
}

func touch(id string) {}
