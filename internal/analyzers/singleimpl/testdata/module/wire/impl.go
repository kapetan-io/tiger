// Package wire implements codec.Encoder — the cross-package half of the
// TS-X01 boundary case.
package wire

import "fixture.example/singleimpl/codec"

// netEncoder is Encoder's only implementation.
type netEncoder struct{}

// Encode renders v's data.
func (netEncoder) Encode(v codec.Value) string {
	return v.Data
}
