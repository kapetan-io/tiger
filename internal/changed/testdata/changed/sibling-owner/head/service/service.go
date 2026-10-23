package service

import (
	"strings"

	"example.com/fakedb"
)

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

// Delete calls another owner that talks to the database, so that call prints.
// The calls to its own methods, to a function that calls no other package and
// to the interface this package declares do not.
func (s *Service) Delete(id string) {
	s.save(id)
	s.trim(id)
	s.audit.Record(normalize(id))
	s.notify.Notify(id)
}

func (s *Service) save(id string) {}

func (s *Service) trim(id string) {
	_ = fakedb.Ping(len(id))
}

func normalize(id string) string {
	return strings.ToLower(id)
}
