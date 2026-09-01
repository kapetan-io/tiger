// Package ts03open declares no restriction at all, so calling through a
// parameter's interface stays silent — closedworld checks nothing without
// a closed-dispatch claim.
package ts03open

// Storage is the interface the call in this file goes through.
type Storage interface {
	Write(entry string)
}

// FromParameter calls through a parameter in a package with no
// closed-dispatch declaration.
func FromParameter(s Storage) {
	s.Write("x")
}
