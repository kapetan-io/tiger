package unviolated_test

import (
	"testing"

	"fixture.example/invariants/assert"
	"fixture.example/invariants/unviolated"
	"fixture.example/invariants/unviolated/inv"
)

// TestEncodeDecodeRoundTrip frames a header and reads it back.
//
// Goal: DecodeHeader returns the sequence EncodeHeader framed.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	decoded := unviolated.DecodeHeader(unviolated.EncodeHeader(512, 7))
	if decoded.Sequence != 7 {
		t.Fatalf("sequence: got %d, want 7", decoded.Sequence)
	}
}

// TestShortBufferViolatesSize decodes an undersized buffer.
//
// Goal: a short buffer fails exactly the header-size invariant.
func TestShortBufferViolatesSize(t *testing.T) {
	assert.Violates(inv.HeaderSize, func() {
		unviolated.DecodeHeader(make([]byte, unviolated.HeaderSizeBytes-1))
	})
}
