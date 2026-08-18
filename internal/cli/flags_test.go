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

// TestCheckDrivesIoinloopPackagesFlag covers the per-repo IO allowlist
// extension: a local subpackage standing in for third-party IO is silent
// by default and only becomes a TS-M10 finding once its import path is
// named on -ioinloop.packages.
//
// Goal: the fixture is clean by default, and -ioinloop.packages naming its
// local storage subpackage turns the per-item call into a blocking finding.
func TestCheckDrivesIoinloopPackagesFlag(t *testing.T) {
	// Analyzer flags are package state shared across runs; reset through the
	// same public surface once the test is done.
	t.Cleanup(func() {
		reset := run(t, "check", "-C", "testdata/fixtures/ioflagged",
			"-ioinloop.packages=", "./...")
		require.Equal(t, cli.ExitClean, reset.code)
	})

	clean := run(t, "check", "-C", "testdata/fixtures/ioflagged", "./...")
	assert.Equal(t, cli.ExitClean, clean.code)
	assert.Empty(t, clean.stdout)

	fired := run(t, "check", "-C", "testdata/fixtures/ioflagged",
		"-ioinloop.packages=fixture.example/ioflagged/storage", "./...")
	assert.Equal(t, cli.ExitFindings, fired.code)
	assert.Contains(t, fired.stdout, "TS-M10")
}
