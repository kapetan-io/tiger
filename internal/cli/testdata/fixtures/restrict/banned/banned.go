// Package banned declares restrictions its own imports contradict: it
// forbids reflect and allows only internal/domain, then imports both
// reflect and a module package outside that subtree.
//
//tiger:restrict no-reflect, imports(internal/domain/...)
package banned

import (
	"reflect"
	"strings"

	"fixture.example/restrict/internal/domain"
	"fixture.example/restrict/other"
)

// Describe names a value's kind through reflection.
func Describe(value domain.Value, thing other.Thing) string {
	return strings.Join([]string{
		reflect.TypeOf(value).Kind().String(),
		reflect.TypeOf(thing).Kind().String(),
	}, "/")
}
