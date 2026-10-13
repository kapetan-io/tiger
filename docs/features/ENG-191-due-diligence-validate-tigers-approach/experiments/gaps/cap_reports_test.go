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

// hugeSpins is a large constant below half of math.MaxInt, so the call-site
// check does not flag it.
const hugeSpins = 1 << 40

func never() bool { return false }

// A page loop over an outside stream stops at its limit with no report needed.
func TestReadLinesStopsAtTheLimit(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("a\nb\nc\nd\ne\n"))
	assert.Equal(t, []string{"a", "b"}, gaps.ReadLines(scanner, 2))
}

// A page loop over an outside stream stops when the stream ends.
func TestReadLinesStopsAtTheEnd(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("a\nb\nc\n"))
	assert.Equal(t, []string{"a", "b", "c"}, gaps.ReadLines(scanner, 10))
}

// A reported cap returns an error when hit.
func TestSpinReportedFailsLoudlyAtTheCap(t *testing.T) {
	require.ErrorContains(t, gaps.SpinReported(never, 1000), "spin reached its limit")
}

// A reported cap returns nil when the loop finishes first.
func TestSpinReportedSucceedsWhenDone(t *testing.T) {
	require.NoError(t, gaps.SpinReported(func() bool { return true }, 1000))
}

// A config value clamped where it enters bounds the loop, which then reports.
func TestSpinFromConfigClampsAHugeValue(t *testing.T) {
	err := gaps.SpinFromConfig(gaps.SpinConfig{Spins: hugeSpins}, never)
	require.ErrorContains(t, err, "spin reached its limit")
}

// Safety caps that run forever when given an unreachable limit. Under call 2
// the first two need a report, the next two are flagged at the call site for
// passing math.MaxInt, the raw config needs a clamp, and the last two are the
// recorded misses.
func TestUnreachableCapsRunForever(t *testing.T) {
	for _, test := range []struct {
		name string
		loop func()
	}{
		{name: "SpinBreak", loop: func() { gaps.SpinBreak(never, math.MaxInt) }},
		{name: "SpinnerField", loop: func() { gaps.Spinner{Max: math.MaxInt}.Spin(never) }},
		// The loop never returns, so there is no error to check.
		{name: "SpinReportedMaxInt", loop: func() { _ = gaps.SpinReported(never, math.MaxInt) }},
		{name: "SpinLimitedMaxInt", loop: func() { gaps.SpinLimited(never, math.MaxInt) }},
		// The loop never returns, so there is no error to check.
		{name: "SpinFromConfigRaw", loop: func() { _ = gaps.SpinFromConfigRaw(gaps.SpinConfig{Spins: hugeSpins}, never) }},
		// The loop never returns, so there is no error to check.
		{name: "SpinReportedHugeConstant", loop: func() { _ = gaps.SpinReported(never, hugeSpins) }},
		{name: "ScanForever", loop: func() { gaps.ScanForever(hugeSpins) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.False(t, finishes(test.loop))
		})
	}
}
