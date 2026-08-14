package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/examples/ledger"
	"github.com/kapetan-io/tiger/examples/ledger/inv"
)

// TestEncodeDecodeRoundTrip frames a header and reads it back.
//
// Goal: DecodeHeader returns the length and sequence EncodeHeader framed.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	decoded := ledger.DecodeHeader(ledger.EncodeHeader(512, 7))
	require.Equal(t, uint32(512), decoded.Length)
	require.Equal(t, uint32(7), decoded.Sequence)
}

// The TS-A09 negative-space proofs: every declared invariant has a test that
// violates it, proving the assertion is reachable and asserted right.

// TestCorruptHeaderViolatesChecksum flips one payload byte (TS-A09).
//
// Goal: corrupt input fails exactly the header-checksum invariant.
func TestCorruptHeaderViolatesChecksum(t *testing.T) {
	assert.Violates(inv.HeaderChecksum, func() {
		encoded := ledger.EncodeHeader(512, 7)
		encoded[5] ^= 0xFF
		ledger.DecodeHeader(encoded)
	})
}

// TestShortBufferViolatesSize decodes an undersized buffer (TS-A09).
//
// Goal: a short buffer fails exactly the header-size invariant.
func TestShortBufferViolatesSize(t *testing.T) {
	assert.Violates(inv.HeaderSize, func() {
		ledger.DecodeHeader(make([]byte, ledger.HeaderSizeBytes-1))
	})
}
