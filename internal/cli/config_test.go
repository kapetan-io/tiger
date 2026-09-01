package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// configTree copies the scoped config fixture and replaces its tiger.yaml
// with entries.
func configTree(t *testing.T, entries string) string {
	t.Helper()
	dir := copyFixture(t, filepath.Join("config", "scoped"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.yaml"), []byte(entries), 0o644))
	return dir
}

// TestConfigScopedEntryAppliesToItsPackages covers acceptance criterion 12.
//
// Goal: a participle.allow entry scoped to the internal/auth subtree
// silences TS-N14 on KeySigning there and the same identifier still fires
// in internal/api.
func TestConfigScopedEntryAppliesToItsPackages(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "config", "scoped"), "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout, "internal/api/api.go:6:6: TS-N14: \"KeySigning\"")
	assert.NotContains(t, got.stdout, "internal/auth")
	assert.Contains(t, got.stdout, "tiger: 1 blocking\n")
}

// TestConfigUnscopedEntryAppliesToTheModule covers acceptance criterion 13.
//
// Goal: an entry with no packages silences the identifier in every package.
func TestConfigUnscopedEntryAppliesToTheModule(t *testing.T) {
	dir := configTree(t, "version: 1\nparticiple:\n  allow:\n    - value: signing\n"+
		"      reason: KeySigning is a credential record\n")
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestConfigRejectsInvalidEntries covers acceptance criteria 14 and 15 and
// invariant I3.
//
// Goal: a missing or empty reason, an unknown analyzer, an unknown key, an
// empty value, a malformed pattern, and an unknown version each exit 2
// with one line naming the offending part, and no finding prints.
func TestConfigRejectsInvalidEntries(t *testing.T) {
	for _, test := range []struct {
		name     string
		entries  string
		wantErrs []string
	}{
		{
			name: "MissingReason",
			entries: "version: 1\nparticiple:\n  allow:\n    - value: signing\n" +
				"      packages: [internal/auth/...]\n",
			wantErrs: []string{"participle", "allow", "signing", "reason"},
		},
		{
			name: "EmptyReason",
			entries: "version: 1\nparticiple:\n  allow:\n    - value: signing\n" +
				"      reason: \"\"\n",
			wantErrs: []string{"participle", "allow", "signing", "reason"},
		},
		{
			name:     "UnknownAnalyzer",
			entries:  "version: 1\nfrobnicate:\n  allow:\n    - value: x\n      reason: y\n",
			wantErrs: []string{"frobnicate"},
		},
		{
			name:     "UnknownKey",
			entries:  "version: 1\nparticiple:\n  frob:\n    - value: x\n      reason: y\n",
			wantErrs: []string{"participle", "frob"},
		},
		{
			name:     "EmptyValue",
			entries:  "version: 1\nparticiple:\n  allow:\n    - value: \"\"\n      reason: y\n",
			wantErrs: []string{"participle", "allow", "value"},
		},
		{
			name: "MalformedPattern",
			entries: "version: 1\nparticiple:\n  allow:\n    - value: signing\n" +
				"      reason: y\n      packages: [../escape]\n",
			wantErrs: []string{"participle", "allow", "../escape"},
		},
		{
			name:     "UnknownVersion",
			entries:  "version: 7\n",
			wantErrs: []string{"version"},
		},
		{
			name:     "NotYAML",
			entries:  "version: [\n",
			wantErrs: []string{"tiger.yaml"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "check", "-C", configTree(t, test.entries), "./...")
			assert.Equal(t, cli.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Contains(t, got.stderr, "tiger.yaml")
			assert.Equal(t, 1, len(splitLines(got.stderr)))
			for _, want := range test.wantErrs {
				assert.Contains(t, got.stderr, want)
			}
		})
	}
}

// TestConfigRejectsFlagOverride covers acceptance criterion 16 and
// behavioral constraint B4.
//
// Goal: an analyzer flag set on the command line while tiger.yaml exists
// exits 2 naming the flag, before any analysis runs.
func TestConfigRejectsFlagOverride(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "config", "scoped"),
		"-participle.allow=x", "./...")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Equal(t, "tiger check: -participle.allow is set on the command line but "+
		"tiger.yaml is the reviewed source — move the value into tiger.yaml\n", got.stderr)
}

// TestConfigDrivesIoinloopPackages covers the ioinloop.packages migration.
//
// Goal: an ioinloop.packages entry names the fixture's storage package and
// the per-item call becomes a TS-M10 finding, exactly as the flag did.
func TestConfigDrivesIoinloopPackages(t *testing.T) {
	dir := copyFixture(t, "ioflagged")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.yaml"), []byte(
		"version: 1\nioinloop:\n  packages:\n    - value: fixture.example/ioflagged/storage\n"+
			"      reason: every Save is a network round trip\n"), 0o644))
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout, "TS-M10")
}

// TestNogoroutineSupervisorsFlagIsGone covers acceptance criterion 19a.
//
// Goal: -nogoroutine.supervisors is an unknown flag, exit 2.
func TestNogoroutineSupervisorsFlagIsGone(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/clean", "-nogoroutine.supervisors=x", "./...")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Contains(t, got.stderr, "flag provided but not defined: -nogoroutine.supervisors")
}

// splitLines returns the non-empty lines of text.
func splitLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
