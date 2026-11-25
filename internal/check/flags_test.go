package check_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

// TestCheckDrivesIoinloopPackagesFlag covers the per-repo -ioinloop.packages
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
		require.Equal(t, command.ExitClean, reset.code)
	})

	clean := run(t, "check", "-C", "testdata/fixtures/ioflagged", "./...")
	assert.Equal(t, command.ExitClean, clean.code)
	assert.Empty(t, clean.stdout)

	fired := run(t, "check", "-C", "testdata/fixtures/ioflagged",
		"-ioinloop.packages=fixture.example/ioflagged/storage", "./...")
	assert.Equal(t, command.ExitFindings, fired.code)
	assert.Contains(t, fired.stdout, "TS-M10")
}
