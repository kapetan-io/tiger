// Package codec declares the encoder abstraction and the value type its
// method signature carries across the package boundary to wire's
// implementation.
package codec

// Value is the payload Encoder methods carry.
type Value struct {
	Data string
}

// Encoder is implemented once, in package wire — proving TS-X01's finish
// step matches across a package boundary, not just within one package.
type Encoder interface { // want `interface Encoder has one implementation, wire\.netEncoder`
	Encode(v Value) string
}
