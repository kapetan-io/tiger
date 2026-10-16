package service

import "example.com/fakedb"

type Audit struct {
	pool *fakedb.Pool
}

func (a *Audit) Record(message string) {
	_ = a.pool.Exec(message)
}

type Notifier interface {
	Notify(message string)
}

type Service struct {
	audit  *Audit
	notify Notifier
}

func (s *Service) Delete(id string) {
	s.save(id)
}

func (s *Service) save(id string) {}
