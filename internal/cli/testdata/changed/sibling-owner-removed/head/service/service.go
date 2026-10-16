package service

import "example.com/fakedb"

type Audit struct {
	pool *fakedb.Pool
}

// Record no longer calls the database, so Delete's old call to it is still a
// removed call: it is judged by what Record did at base.
func (a *Audit) Record(message string) {}

func (a *Audit) Mute() {}

type Service struct {
	audit *Audit
}

func (s *Service) Delete(id string) {}

// Note starts calling a function of another owner that calls no other
// package, so nothing prints for it.
func (s *Service) Note(id string) {
	s.audit.Mute()
}
