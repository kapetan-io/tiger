// Package singlesite mirrors examples/ledger's header pair with one deliberate
// change to what it asserts.
package singlesite

import (
	"encoding/binary"
	"hash/crc32"

	"fixture.example/invariants/assert"
	"fixture.example/invariants/singlesite/inv"
)

// HeaderSizeBytes = 4 checksum + 4 length + 4 sequence.
const HeaderSizeBytes = 12

// Header frames one batch in the ledger's log.
type Header struct {
	Checksum uint32
	Length   uint32
	Sequence uint32
}

// Length counts the payload bytes a header frames.
type Length uint32

// Sequence orders frames within the log.
type Sequence uint32

// EncodeHeader frames length and sequence into a HeaderSizeBytes buffer.
func EncodeHeader(length Length, sequence Sequence) []byte {
	target := make([]byte, HeaderSizeBytes)
	binary.BigEndian.PutUint32(target[4:8], uint32(length))
	binary.BigEndian.PutUint32(target[8:12], uint32(sequence))
	binary.BigEndian.PutUint32(target[0:4], crc32.ChecksumIEEE(target[4:12]))
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
