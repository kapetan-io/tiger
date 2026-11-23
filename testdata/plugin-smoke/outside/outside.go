// Package outside is outside the tiger.yaml entry's package scope, so its
// comments must surface as TS-L14 through the plugin.
package outside

// KeySigning is a credential record.
type KeySigning struct{}
