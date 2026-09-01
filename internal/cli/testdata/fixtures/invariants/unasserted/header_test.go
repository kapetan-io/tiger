package unasserted_test

import (
	"testing"

	"fixture.example/invariants/assert"
	"fixture.example/invariants/unasserted"
	"fixture.example/invariants/unasserted/inv"
)

// TestEncodeDecodeRoundTrip frames a header and reads it back.
//
// Goal: DecodeHeader returns the sequence EncodeHeader framed.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	decoded := unasserted.DecodeHeader(unasserted.EncodeHeader(512, 7))
	if decoded.Sequence != 7 {
		t.Fatalf("sequence: got %d, want 7", decoded.Sequence)
	}
}

// TestCorruptHeaderViolatesChecksum flips one payload byte.
//
// Goal: corrupt input fails exactly the header-checksum invariant.
func TestCorruptHeaderViolatesChecksum(t *testing.T) {
	assert.Violates(inv.HeaderChecksum, func() {
		encoded := unasserted.EncodeHeader(512, 7)
		encoded[5] ^= 0xFF
		unasserted.DecodeHeader(encoded)
	})
}

// TestShortBufferViolatesSize decodes an undersized buffer.
//
// Goal: a short buffer fails exactly the header-size invariant.
func TestShortBufferViolatesSize(t *testing.T) {
	assert.Violates(inv.HeaderSize, func() {
		unasserted.DecodeHeader(make([]byte, unasserted.HeaderSizeBytes-1))
	})
}
