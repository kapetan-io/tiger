// Package scoped is inside the tiger.yaml entry's package scope, so
// KeySigning must not surface as TS-N14 through the plugin.
package scoped

// KeySigning is a credential record.
type KeySigning struct{}
