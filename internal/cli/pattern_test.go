package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// TestPatternPrintsCardCodeAndTests prints a pattern from the binary.
//
// Goal: tiger pattern ledger prints the card sections, then the code, then
// the tests, and exits 0.
func TestPatternPrintsCardCodeAndTests(t *testing.T) {
	got := run(t, "pattern", "ledger")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stderr)
	for _, want := range []string{
		"# ledger", "Rules: TS-A07, TS-A09", "Gap: ", "Bug: ", "Checklist:\n- ",
		"## Code: ledger/header.go", "## Code: ledger/inv/inv.go",
		"## Tests: ledger/header_test.go",
	} {
		assert.Contains(t, got.stdout, want)
	}
	assert.Less(t, strings.Index(got.stdout, "## Code: ledger/header.go"),
		strings.Index(got.stdout, "## Tests: ledger/header_test.go"))
}

// TestPatternAcceptsFlagsBeforeOrAfterTheName covers both argument orders.
//
// Goal: --checklist prints only the reviewer questions whether it comes before
// or after the pattern name.
func TestPatternAcceptsFlagsBeforeOrAfterTheName(t *testing.T) {
	after := run(t, "pattern", "ledger", "--checklist")
	before := run(t, "pattern", "--checklist", "ledger")
	assert.Equal(t, cli.ExitClean, after.code)
	assert.Equal(t, after.stdout, before.stdout)
	assert.True(t, strings.HasPrefix(after.stdout, "ledger\n- "))
	assert.NotContains(t, after.stdout, "package ledger")
}

// TestPatternRuleListsThePatternsBehindARule covers --rule.
//
// Goal: --rule TS-C02 lists both patterns that back it, one per line, and
// --rule with --checklist prints each one's questions.
func TestPatternRuleListsThePatternsBehindARule(t *testing.T) {
	listed := run(t, "pattern", "--rule", "TS-C02")
	assert.Equal(t, cli.ExitClean, listed.code)
	lines := strings.Split(strings.TrimSpace(listed.stdout), "\n")
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], "goroutine-owner\t"))
	assert.True(t, strings.HasPrefix(lines[1], "shutdown-loop\t"))

	questions := run(t, "pattern", "--rule", "TS-C02", "--checklist")
	assert.Equal(t, cli.ExitClean, questions.code)
	assert.Contains(t, questions.stdout, "goroutine-owner\n- ")
	assert.Contains(t, questions.stdout, "shutdown-loop\n- ")
}

// TestPatternWithoutArgumentsListsEveryPattern covers the index.
//
// Goal: with no arguments tiger pattern lists the nine patterns by name.
func TestPatternWithoutArgumentsListsEveryPattern(t *testing.T) {
	got := run(t, "pattern")
	assert.Equal(t, cli.ExitClean, got.code)
	names := []string{}
	for _, line := range strings.Split(strings.TrimSpace(got.stdout), "\n") {
		name, _, _ := strings.Cut(line, "\t")
		names = append(names, name)
	}
	assert.Equal(t, []string{
		"capped-worklist", "clamped-limit", "filtered-page", "goroutine-owner", "ledger",
		"pool-reuse", "shutdown-loop", "sorted-map-output", "stream-page",
	}, names)
}

// TestPatternRefusesWhatItCannotAnswer covers the operational failures.
//
// Goal: an unknown pattern, an unknown rule, and a bad flag combination exit 2
// with a message on stderr and nothing on stdout.
func TestPatternRefusesWhatItCannotAnswer(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "UnknownPattern", args: []string{"pattern", "nope"}, want: `unknown pattern "nope"`},
		{
			name: "UnknownRule", args: []string{"pattern", "--rule", "TS-Z99"},
			want: "no pattern backs TS-Z99",
		},
		{
			name: "RuleAndName", args: []string{"pattern", "ledger", "--rule", "TS-A07"},
			want: "cannot be combined",
		},
		{
			name: "ChecklistAlone", args: []string{"pattern", "--checklist"},
			want: "needs a pattern name",
		},
		{
			name: "TwoNames", args: []string{"pattern", "ledger", "extra"},
			want: "at most one pattern name",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, test.args...)
			assert.Equal(t, cli.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			require.NotEmpty(t, got.stderr)
			assert.Contains(t, got.stderr, test.want)
		})
	}
}

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
