// Package outside is outside the tiger.yaml entry's package scope.
package outside

// KeySigning is deliberately named with a trailing participle.
type KeySigning struct{} // want `TS-N14`
