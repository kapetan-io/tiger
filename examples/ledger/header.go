// Package ledger demonstrates the Tiger Go assertion and invariant pattern:
// invariants declared once in an inv package (TS-A07), asserted at both ends
// of a symmetric encode/decode pair (TS-A08), and violated on purpose by a
// test per invariant (TS-A09, see header_test.go).
package ledger

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/examples/ledger/inv"
)

// HeaderSizeBytes = 4 checksum + 4 length + 4 sequence.
const HeaderSizeBytes = 12

// Header frames one batch in the ledger's log.
type Header struct {
	Checksum uint32
	Length   uint32
	Sequence uint32
}

// Length counts the payload bytes a header frames (TS-Q01: domain
// quantities are named types, so a swapped argument is a compile error).
type Length uint32

// Sequence orders frames within the log (TS-Q01).
type Sequence uint32

// EncodeHeader frames length and sequence into a HeaderSizeBytes buffer,
// computing the checksum over the payload fields.
func EncodeHeader(length Length, sequence Sequence) []byte {
	target := make([]byte, HeaderSizeBytes)
	binary.BigEndian.PutUint32(target[4:8], uint32(length))
	binary.BigEndian.PutUint32(target[8:12], uint32(sequence))
	binary.BigEndian.PutUint32(target[0:4], crc32.ChecksumIEEE(target[4:12]))
	assert.Invariant(inv.HeaderSize, len(target) == HeaderSizeBytes)
	assert.Invariant(inv.HeaderChecksum,
		binary.BigEndian.Uint32(target[0:4]) == crc32.ChecksumIEEE(target[4:12]))
	return target
}

// DecodeHeader reads a Header back out of target, refusing corrupt input.
func DecodeHeader(target []byte) Header {
	assert.Invariant(inv.HeaderSize, len(target) == HeaderSizeBytes)
	header := Header{
		Checksum: binary.BigEndian.Uint32(target[0:4]),
		Length:   binary.BigEndian.Uint32(target[4:8]),
		Sequence: binary.BigEndian.Uint32(target[8:12]),
	}
	assert.Invariant(inv.HeaderChecksum, header.Checksum == crc32.ChecksumIEEE(target[4:12]))
	return header
}
