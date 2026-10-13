package gaps_test

import (
	"bufio"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaps"
)

// endlessRows is a stream with more rows than any page.
func endlessRows() *bufio.Scanner {
	return bufio.NewScanner(strings.NewReader(strings.Repeat("row\n", 5_000)))
}

// A clamped request limit returns at most listMax rows.
func TestListClampedStopsAtListMax(t *testing.T) {
	assert.Equal(t, 1_000, gaps.ListClamped(endlessRows(), gaps.ListRequest{Limit: math.MaxInt32}))
}

// A raw request limit lets the client take every row.
func TestListRawReturnsWhatTheClientAsks(t *testing.T) {
	assert.Equal(t, 5_000, gaps.ListRaw(endlessRows(), gaps.ListRequest{Limit: math.MaxInt32}))
}

// header is a 12-byte pack header claiming count objects and holding none.
func header(count uint32) []byte {
	return []byte{'P', 'A', 'C', 'K', 0, 0, 0, 2,
		byte(count >> 24), byte(count >> 16), byte(count >> 8), byte(count)}
}

// The raw pack parser allocates room for whatever count the header claims.
func TestParsePackRawAllocatesTheClaimedCount(t *testing.T) {
	entries := gaps.ParsePackRaw(header(1 << 22))
	assert.Equal(t, 1<<22, cap(entries))
	assert.Empty(t, entries)
}

// The bounded pack parser rejects a count the pack cannot hold.
func TestParsePackRejectsAnImpossibleCount(t *testing.T) {
	_, err := gaps.ParsePack(header(1 << 22))
	require.ErrorContains(t, err, "pack claims more objects than it can hold")
}

// The bounded pack parser accepts an honest pack.
func TestParsePackAcceptsAnHonestPack(t *testing.T) {
	entries, err := gaps.ParsePack(append(header(3), 'a', 'b', 'c'))
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

// Partitions clamped in a helper stop at partitionsMax.
func TestCreateClampedStopsAtPartitionsMax(t *testing.T) {
	assert.Equal(t, 256, gaps.CreateClamped(math.MaxInt))
}

// Loops whose limit enters without a bound and run forever. CreateRaw is
// flagged under change 6 with the bounded-result refinement; SpinThroughValue
// is the recorded miss for calls through function values.
func TestUnboundedEntriesRunForever(t *testing.T) {
	for _, test := range []struct {
		name string
		loop func()
	}{
		{name: "CreateRaw", loop: func() { gaps.CreateRaw(math.MaxInt) }},
		// The loop never returns, so there is no error to check.
		{name: "SpinThroughValue", loop: func() { _ = gaps.SpinThroughValue(gaps.SpinConfig{Spins: hugeSpins}, never) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.False(t, finishes(test.loop))
		})
	}
}
