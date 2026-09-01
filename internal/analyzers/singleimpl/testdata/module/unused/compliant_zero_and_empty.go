// Package unused declares an interface with no implementations and an
// empty interface, both compliant: zero implementations is not a finding,
// and an interface with no methods is excluded by rule.
package unused

// Sink has no implementations yet.
type Sink interface {
	Write(data []byte) (int, error)
}

// Marker has no methods, so nothing can fail to implement it — TS-X01
// excludes it entirely, never counting it as an interface to check.
type Marker interface{}
