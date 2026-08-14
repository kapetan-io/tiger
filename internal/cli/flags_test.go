package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// TestCheckDrivesAnalyzerFlags covers the configuration contract: analyzer
// configuration rides the standard per-analyzer flag mechanism, prefixed
// with the analyzer's name on the check command line.
//
// Goal: a finding that fires by default is silenced by the analyzer's own
// allowlist flag passed through tiger check.
func TestCheckDrivesAnalyzerFlags(t *testing.T) {
	// Analyzer flags are package state shared across runs; reset through the
	// same public surface once the test is done.
	t.Cleanup(func() {
		reset := run(t, "check", "-C", "testdata/fixtures/flagged", "-participle.allow=", "./...")
		require.Equal(t, cli.ExitFindings, reset.code)
	})

	fired := run(t, "check", "-C", "testdata/fixtures/flagged", "./...")
	assert.Equal(t, cli.ExitFindings, fired.code)
	assert.Contains(t, fired.stdout, "TS-N14")
	assert.Contains(t, fired.stdout, "Preparing")

	allowed := run(t, "check", "-C", "testdata/fixtures/flagged",
		"-participle.allow=preparing", "./...")
	assert.Equal(t, cli.ExitClean, allowed.code)
	assert.Empty(t, allowed.stdout)
}
