// Package fixture proves the -allow flag extends the default noun
// allowlist with a project-owned term (timing) on top of the built-in
// ones.
package fixture

// Timing is allowlisted once -allow=timing is set.
type Timing struct{}

// Preparing is not allowlisted, so it still fires.
type Preparing struct{} // want `TS-N14: "Preparing" ends in the participle "Preparing"`

// String is still silent through the built-in default allowlist, proving
// the flag extends rather than replaces it.
type String struct{}
