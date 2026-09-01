// Package unrelated declares a named string type that no assert call ever
// names — not an invariant, so TS-A07 stays silent on it (compliant case:
// absence of a claim is never a finding).
package unrelated

// Label is a named string type, never passed to assert.Invariant or
// assert.Violates anywhere in the module.
type Label string

const Unused Label = "unused"
