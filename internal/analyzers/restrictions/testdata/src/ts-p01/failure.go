// Package tsp01 declares restrictions its own imports contradict, and a
// second directive contradicting the first (see failure2.go): both fire
// TS-P01.
//
//tiger:restrict no-reflect, imports(fixture.example/ts-p01helper/domain/...)
package tsp01

import (
	"reflect" // want "TS-P01: package tsp01 declares //tiger:restrict no-reflect but imports reflect"

	"fixture.example/ts-p01other" // want `TS-P01: package tsp01 imports fixture\.example/ts-p01other, which its //tiger:restrict imports\(\.\.\.\) list does not allow`
)

// UseReflect and UseOther exist only so the two imports above are used —
// an unused import is a compile error, not a TS-P01 finding.
func UseReflect() reflect.Type {
	return reflect.TypeOf(0)
}

// UseOther references the disallowed dependency.
func UseOther() string {
	return other.Marker
}
