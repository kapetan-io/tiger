// Package reason is a TS-S08 corpus support package: Code is marked
// //tiger:openenum here, in its own package, to exercise the cross-package
// limitation exercised from testdata/src/ts-s08 (see
// failure_crosspackage_openenum.go). compoundcond only reads directives
// from the package it is analyzing, so this marking is invisible to it.
package reason

// Code is deliberately open: the wire can carry a value this package
// hasn't named yet.
//
//tiger:openenum
type Code string

const (
	CodeOne Code = "one"
	CodeTwo Code = "two"
)
