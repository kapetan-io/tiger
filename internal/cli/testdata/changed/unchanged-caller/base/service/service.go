package service

import "example.com/fakedb"

type Service struct {
	pool *fakedb.Pool
}

func (s *Service) Handle(id string) {
	stamp(id)
}

func (s *Service) Other(id string) {
	s.pool.Close()
	touch(id)
}

func stamp(id string) {}

func touch(id string) {
	_ = fakedb.Ping(len(id))
}
