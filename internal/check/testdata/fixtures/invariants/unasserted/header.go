// Package unasserted mirrors patterns/ledger's header pair with one deliberate
// change to what it asserts.
package unasserted

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/unasserted/inv"
)


const HeaderSizeBytes = 12


type Header struct {
	Checksum uint32
	Length   uint32
	Sequence uint32
}


type Length uint32


type Sequence uint32


func EncodeHeader(length Length, sequence Sequence) []byte {
	target := make([]byte, HeaderSizeBytes)
	binary.BigEndian.PutUint32(target[4:8], uint32(length))
	binary.BigEndian.PutUint32(target[8:12], uint32(sequence))
	binary.BigEndian.PutUint32(target[0:4], crc32.ChecksumIEEE(target[4:12]))
	assert.Invariant(inv.HeaderChecksum,
		binary.BigEndian.Uint32(target[0:4]) == crc32.ChecksumIEEE(target[4:12]))
	return target
}


func DecodeHeader(target []byte) Header {
	header := Header{
		Checksum: binary.BigEndian.Uint32(target[0:4]),
		Length:   binary.BigEndian.Uint32(target[4:8]),
		Sequence: binary.BigEndian.Uint32(target[8:12]),
	}
	assert.Invariant(inv.HeaderChecksum, header.Checksum == crc32.ChecksumIEEE(target[4:12]))
	return header
}
