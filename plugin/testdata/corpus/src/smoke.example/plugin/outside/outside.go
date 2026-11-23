// Package outside is outside the tiger.yaml entry's package scope.
package outside

type KeySigning struct{} // want `TS-L14`
