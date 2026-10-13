// Package lib holds loops whose limits come from their callers.
package lib

import "errors"

var errCapHit = errors.New("cap hit")

// SpinReported spins until done or limit.
func SpinReported(done func() bool, limit int) error { // want SpinReported:`\[1\]`
	i := 0
	for ; i < limit && !done(); i++ {
	}
	if i == limit {
		return errCapHit
	}
	return nil
}

// Spin has the limit in its header.
func Spin(done func() bool, limit int) error { // want Spin:`\[1\]`
	for i := 0; i < limit; i++ {
		if done() {
			return nil
		}
	}
	return errCapHit
}
