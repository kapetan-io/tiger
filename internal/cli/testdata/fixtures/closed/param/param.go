// Package param claims closed dispatch but calls through an interface
// whose receiver is a parameter, which no single concrete type resolves.
//
//tiger:restrict closed-dispatch
package param

// Storage persists bytes.
type Storage interface {
	Write(data []byte) int
}

// Save writes through whatever storage the caller passed.
func Save(s Storage, data []byte) int {
	return s.Write(data)
}
