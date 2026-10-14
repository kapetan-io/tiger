package ledger

import (
	"encoding/binary"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/patterns/ledger/inv"
)

// DecodeHeaderUnchecked is the broken shape: it asserts the size and never the
// checksum. Every invariant is still asserted in production code (TS-A07) and
// still violated by a test (TS-A09), so tiger check passes, yet a corrupted
// frame decodes without complaint.
func DecodeHeaderUnchecked(target []byte) Header {
	assert.Invariant(inv.HeaderSize, len(target) == HeaderSizeBytes)
	return Header{
		Checksum: binary.BigEndian.Uint32(target[0:4]),
		Length:   binary.BigEndian.Uint32(target[4:8]),
		Sequence: binary.BigEndian.Uint32(target[8:12]),
	}
}
