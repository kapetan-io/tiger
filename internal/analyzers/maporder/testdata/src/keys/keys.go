// Package keys holds key types whose comparators other packages use, to
// test that the total-compare fact crosses package boundaries and that
// unexported fields can only be covered from inside.
package keys

import (
	"cmp"
	"strings"
)

type ID struct {
	Zone string
	Num  uint32
}

func (i ID) Compare(o ID) int { // want Compare:"total"
	return cmp.Or(strings.Compare(i.Zone, o.Zone), cmp.Compare(i.Num, o.Num))
}

// Opaque has an unexported field; Order covers it, under any method name.
type Opaque struct {
	Name   string
	secret int
}

func NewOpaque(name string, secret int) Opaque { return Opaque{name, secret} }

func (o Opaque) Order(p Opaque) int { // want Order:"total"
	return cmp.Or(cmp.Compare(o.Name, p.Name), cmp.Compare(o.secret, p.secret))
}

// Partial's Compare skips the unexported field.
type Partial struct {
	Name   string
	secret int
}

func (p Partial) Compare(q Partial) int { return strings.Compare(p.Name, q.Name) }
