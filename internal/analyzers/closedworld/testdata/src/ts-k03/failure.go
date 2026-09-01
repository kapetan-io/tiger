// Package tsk03 declares closed dispatch but calls through interfaces
// whose receivers do not devirtualize to one concrete type: a parameter,
// a struct field, a call result, two concrete types meeting at a Phi, and
// a standard-library interface method — closed-dispatch exempts none of
// them.
//
//tiger:restrict closed-dispatch
package tsk03

// Storage is the interface every failing call in this file goes through.
type Storage interface {
	Write(entry string)
}

type diskStorage struct{}

func (diskStorage) Write(string) {}

type memStorage struct{}

func (memStorage) Write(string) {}

// holder carries a Storage field — closedworld cannot see through a
// field read, so a call on it is a finding.
type holder struct {
	store Storage
}

// FromParameter calls through a parameter — the open-world default this
// analyzer must still catch under closed-dispatch.
func FromParameter(s Storage) {
	s.Write("x") // want `TS-K03: s\.Write is called through interface Storage in a package that declares //tiger:restrict closed-dispatch`
}

// FromField calls through a struct field.
func FromField(h holder) {
	h.store.Write("x") // want `TS-K03: h\.store\.Write is called through interface Storage in a package that declares //tiger:restrict closed-dispatch`
}

func newStorage() Storage {
	return diskStorage{}
}

// FromCallResult calls through the result of another function —
// closedworld does not chase returns.
func FromCallResult() {
	newStorage().Write("x") // want `TS-K03: newStorage\(\)\.Write is called through interface Storage in a package that declares //tiger:restrict closed-dispatch`
}

// FromPhi builds the receiver from two different concrete types across a
// branch, so the Phi node they meet at carries no single answer.
func FromPhi(pick bool) {
	var s Storage
	if pick {
		s = diskStorage{}
	} else {
		s = memStorage{}
	}
	s.Write("x") // want `TS-K03: s\.Write is called through interface Storage in a package that declares //tiger:restrict closed-dispatch`
}

// FromStdlib calls a standard-library interface method on a parameter —
// closed-dispatch is not exempt for error.Error, io.Reader.Read, or any
// other stdlib interface.
func FromStdlib(err error) string {
	return err.Error() // want `TS-K03: err\.Error is called through interface error in a package that declares //tiger:restrict closed-dispatch`
}
