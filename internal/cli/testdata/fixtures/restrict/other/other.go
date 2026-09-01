// Package other is a module package outside the allowed import subtree. It
// declares the same axes banned claims, so the only finding its import
// causes is the TS-P01 at the import spec.
//
//tiger:restrict no-reflect, imports(internal/domain/...)
package other

// Thing is exported so banned can import this package.
type Thing int
