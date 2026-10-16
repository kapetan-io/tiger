package service

import "example.com/fakedb"

type Audit struct {
	pool *fakedb.Pool
}

func (a *Audit) Record(message string) {
	_ = a.pool.Exec(message)
}

func (a *Audit) Mute() {}

type Service struct {
	audit *Audit
}

func (s *Service) Delete(id string) {
	s.audit.Record(id)
}

func (s *Service) Note(id string) {}
