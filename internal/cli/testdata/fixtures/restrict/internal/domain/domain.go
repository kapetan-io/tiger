// Package domain is inside the allowed import subtree. It declares the same
// axes banned claims, so it weakens nothing.
//
//tiger:restrict no-reflect, imports(internal/domain/...)
package domain

// Value is a domain value.
type Value int
