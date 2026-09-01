// Package outside is outside the tiger.yaml entry's package scope, so
// KeySigning must surface as TS-N14 through the plugin.
package outside

// KeySigning is deliberately named with a trailing participle.
type KeySigning struct{}
