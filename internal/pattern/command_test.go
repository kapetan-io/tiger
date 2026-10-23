package pattern_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/pattern"
)

// outcome is one invocation's observable result.
type outcome struct {
	code   int
	stdout string
	stderr string
}

// run invokes tiger pattern once. The leading word keeps each call site
// reading as the command line it stands for.
func run(t *testing.T, args ...string) outcome {
	t.Helper()
	require.Equal(t, "pattern", args[0])
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := pattern.Run(args[1:], command.Streams{Stdout: stdout, Stderr: stderr})
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// TestPatternPrintsCardCodeAndTests prints a pattern from the binary.
//
// Goal: tiger pattern ledger prints the card sections, then the code, then
// the tests, and exits 0.
func TestPatternPrintsCardCodeAndTests(t *testing.T) {
	got := run(t, "pattern", "ledger")
	assert.Equal(t, command.ExitClean, got.code)
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
	assert.Equal(t, command.ExitClean, after.code)
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
	assert.Equal(t, command.ExitClean, listed.code)
	lines := strings.Split(strings.TrimSpace(listed.stdout), "\n")
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], "goroutine-owner\t"))
	assert.True(t, strings.HasPrefix(lines[1], "shutdown-loop\t"))

	questions := run(t, "pattern", "--rule", "TS-C02", "--checklist")
	assert.Equal(t, command.ExitClean, questions.code)
	assert.Contains(t, questions.stdout, "goroutine-owner\n- ")
	assert.Contains(t, questions.stdout, "shutdown-loop\n- ")
}

// TestPatternWithoutArgumentsListsEveryPattern covers the index.
//
// Goal: with no arguments tiger pattern lists the nine patterns by name.
func TestPatternWithoutArgumentsListsEveryPattern(t *testing.T) {
	got := run(t, "pattern")
	assert.Equal(t, command.ExitClean, got.code)
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
			assert.Equal(t, command.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			require.NotEmpty(t, got.stderr)
			assert.Contains(t, got.stderr, test.want)
		})
	}
}

// shippedPatterns is the seed collection plus the ledger (ENG-342).
var shippedPatterns = []string{
	"capped-worklist", "clamped-limit", "filtered-page", "goroutine-owner", "ledger",
	"pool-reuse", "shutdown-loop", "sorted-map-output", "stream-page",
}

// TestPatternPrintsEveryShippedPatternWhole prints each pattern by name.
//
// Goal: every shipped pattern prints its rules, gap, bug, a checklist of at
// least three questions, a Go file as code and a Go test file as tests.
func TestPatternPrintsEveryShippedPatternWhole(t *testing.T) {
	for _, name := range shippedPatterns {
		t.Run(name, func(t *testing.T) {
			got := run(t, "pattern", name)
			require.Equal(t, command.ExitClean, got.code)
			assert.Empty(t, got.stderr)
			assert.True(t, strings.HasPrefix(got.stdout, "# "+name+"\n"))
			assert.Regexp(t, `Rules: TS-[A-Z]+[0-9]+`, got.stdout)
			assert.Regexp(t, `Gap: \S`, got.stdout)
			assert.Regexp(t, `Bug: \S`, got.stdout)
			assert.Contains(t, got.stdout, "\n## Code: "+name+"/")
			assert.Contains(t, got.stdout, "\n## Tests: "+name+"/")
			assert.Contains(t, got.stdout, "_test.go")

			checklist := run(t, "pattern", name, "--checklist")
			require.Equal(t, command.ExitClean, checklist.code)
			lines := strings.Split(strings.TrimSpace(checklist.stdout), "\n")
			require.GreaterOrEqual(t, len(lines), 4)
			assert.Equal(t, name, lines[0])
			for _, line := range lines[1:] {
				assert.True(t, strings.HasPrefix(line, "- "), line)
				assert.True(t, strings.HasSuffix(line, "?"), line)
			}
		})
	}
}

// TestPatternHelpPrintsUsageThenFlagDefaults reads tiger pattern -h.
//
// Goal: -h prints "usage:", the command's listing entry, then the flag
// defaults, all on stderr, and exits 2.
func TestPatternHelpPrintsUsageThenFlagDefaults(t *testing.T) {
	got := run(t, "pattern", "-h")
	assert.Equal(t, command.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.True(t, strings.HasPrefix(got.stderr, "usage:\n"+pattern.Usage))
	assert.Contains(t, got.stderr, "  -checklist\n")
	assert.Contains(t, got.stderr, "  -rule string\n")
}
