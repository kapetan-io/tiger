package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thrawn01/tiger/assert"
	"github.com/thrawn01/tiger/examples/ledger"
	"github.com/thrawn01/tiger/examples/ledger/inv"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	decoded := ledger.DecodeHeader(ledger.EncodeHeader(512, 7))
	require.Equal(t, uint32(512), decoded.Length)
	require.Equal(t, uint32(7), decoded.Sequence)
}

// The TS-A09 negative-space proofs: every declared invariant has a test that
// violates it, proving the assertion is reachable and asserted right.

func TestCorruptHeaderViolatesChecksum(t *testing.T) {
	assert.Violates(inv.HeaderChecksum, func() {
		encoded := ledger.EncodeHeader(512, 7)
		encoded[5] ^= 0xFF
		ledger.DecodeHeader(encoded)
	})
}

func TestShortBufferViolatesSize(t *testing.T) {
	assert.Violates(inv.HeaderSize, func() {
		ledger.DecodeHeader(make([]byte, ledger.HeaderSizeBytes-1))
	})
}
