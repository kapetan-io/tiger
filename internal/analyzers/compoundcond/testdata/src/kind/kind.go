// Package kind is a TS-S08 corpus support package. It declares a named type
// with no constants of its own — the constants live in the ts-s08 fixture
// package instead, so the type's own package scope has nothing to find.
package kind

// Status is a closed-looking set whose constants are declared outside this
// package.
type Status int
