package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// TestCheckFindingEndsWithThePatternPointer covers the driver's suffix.
//
// Goal: a TS-C02 finding ends with the list of patterns that back it, and a
// finding from a rule no pattern backs is printed unchanged.
func TestCheckFindingEndsWithThePatternPointer(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/pointed", "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	lines := strings.Split(got.stdout, "\n")
	require.Greater(t, len(lines), 1)
	assert.Contains(t, lines[0], ": TS-C02: ")
	assert.True(t, strings.HasSuffix(lines[0], " — see `tiger pattern --rule TS-C02`"))

	plain := run(t, "check", "-C", "testdata/fixtures/findings", "./...")
	assert.NotContains(t, plain.stdout, "tiger pattern")
}
