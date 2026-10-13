// Package wire decodes numbers off the wire.
package wire

// Uint32 reads a big-endian number from the first four bytes.
func Uint32(raw []byte) uint32 {
	return uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
}
