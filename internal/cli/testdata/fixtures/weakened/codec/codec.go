// Package codec claims closed-dispatch and nothing else, so it weakens any
// importer's no-reflect claim.
//
//tiger:restrict closed-dispatch
package codec

// Encode doubles every byte.
func Encode(data []byte) []byte {
	out := make([]byte, 0, len(data)*2)
	for i := 0; i < len(data); i++ {
		out = append(out, data[i], data[i])
	}
	return out
}
