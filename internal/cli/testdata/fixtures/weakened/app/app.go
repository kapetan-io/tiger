// Package app claims closed-dispatch and no-reflect, and imports two
// packages that each claim only one of them — so each axis is weakened
// by a different dependency.
//
//tiger:restrict closed-dispatch, no-reflect
package app

import (
	"fixture.example/weakened/codec"
	"fixture.example/weakened/store"
)

// Run encodes then stores.
func Run(data []byte) int {
	return store.Put(codec.Encode(data))
}
