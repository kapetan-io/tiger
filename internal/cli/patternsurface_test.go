package cli_test

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
	"github.com/kapetan-io/tiger/internal/rules"
)

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
			require.Equal(t, cli.ExitClean, got.code)
			assert.Empty(t, got.stderr)
			assert.True(t, strings.HasPrefix(got.stdout, "# "+name+"\n"))
			assert.Regexp(t, `Rules: TS-[A-Z]+[0-9]+`, got.stdout)
			assert.Regexp(t, `Gap: \S`, got.stdout)
			assert.Regexp(t, `Bug: \S`, got.stdout)
			assert.Contains(t, got.stdout, "\n## Code: "+name+"/")
			assert.Contains(t, got.stdout, "\n## Tests: "+name+"/")
			assert.Contains(t, got.stdout, "_test.go")

			checklist := run(t, "pattern", name, "--checklist")
			require.Equal(t, cli.ExitClean, checklist.code)
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

// TestPatternRuleAnswersMatchTheRegistry asks the binary for the patterns
// behind every rule the registry points at a pattern.
//
// Goal: for each rule code, tiger pattern --rule lists exactly the patterns
// the registry names for that code, so a finding's pointer never leads to an
// empty or different answer.
func TestPatternRuleAnswersMatchTheRegistry(t *testing.T) {
	want := map[string]map[string]bool{}
	for _, rule := range rules.CustomRules() {
		for _, name := range rule.Patterns {
			if want[rule.RuleID] == nil {
				want[rule.RuleID] = map[string]bool{}
			}
			want[rule.RuleID][name] = true
		}
	}
	require.NotEmpty(t, want)
	for _, code := range slices.Sorted(maps.Keys(want)) {
		names := want[code]
		t.Run(code, func(t *testing.T) {
			got := run(t, "pattern", "--rule", code)
			require.Equal(t, cli.ExitClean, got.code)
			listed := map[string]bool{}
			for _, line := range strings.Split(strings.TrimSpace(got.stdout), "\n") {
				name, _, _ := strings.Cut(line, "\t")
				listed[name] = true
			}
			assert.Equal(t, names, listed)
		})
	}
}

// TestPatternHelpNamesTheCommand reads the usage text.
//
// Goal: tiger help documents tiger pattern with its two flags.
func TestPatternHelpNamesTheCommand(t *testing.T) {
	got := run(t, "help")
	assert.Contains(t, got.stdout, "tiger pattern [name] [--checklist] [--rule code]")
}

// TestCheckOnThePatternsTreeIsSilent runs tiger check over the collection.
//
// Goal: every pattern, broken shapes included, passes tiger check with zero
// findings, the way the CI patterns job demands.
func TestCheckOnThePatternsTreeIsSilent(t *testing.T) {
	got := run(t, "check", "-C", "../..", "./patterns/...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

var findingLine = regexp.MustCompile(`^\S+: (TS-[A-Z]+[0-9]+): (.*)$`)

// TestCheckFindingsEndWithTheirPointerWithinTheCap reads every finding the
// fixtures produce.
//
// Goal: every printed finding ends with the pointer the registry defines for
// its rule (one pattern, several, or none), and the body of a pointed
// finding, pointer included, stays within 240 characters. The fixtures must exercise a one-pattern
// pointer, a several-pattern pointer, and a rule with no pattern.
func TestCheckFindingsEndWithTheirPointerWithinTheCap(t *testing.T) {
	const maxBodyRunes = 240
	possible := map[string]map[string]bool{}
	for _, rule := range rules.CustomRules() {
		if possible[rule.RuleID] == nil {
			possible[rule.RuleID] = map[string]bool{}
		}
		possible[rule.RuleID][rules.Pointer(rule.Category)] = true
	}
	entries, err := os.ReadDir("testdata/fixtures")
	require.NoError(t, err)
	fixtures := []string{}
	for _, entry := range entries {
		fixtures = append(fixtures, "testdata/fixtures/"+entry.Name())
	}
	fixtures = append(fixtures, "testdata/fixtures/invariants/unasserted",
		"testdata/fixtures/invariants/unviolated", "testdata/fixtures/invariants/singlesite")

	forms := map[string]bool{}
	for _, dir := range fixtures {
		got := run(t, "check", "-C", dir, "./...")
		if got.code != cli.ExitFindings {
			continue
		}
		for _, line := range strings.Split(got.stdout, "\n") {
			match := findingLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			code, body := match[1], match[2]
			pointer := ""
			if at := strings.Index(body, " — see `tiger pattern"); at >= 0 {
				pointer = body[at:]
			}
			if _, registered := possible[code]; registered {
				assert.True(t, possible[code][pointer], "%s: %s", dir, line)
			}
			if pointer != "" {
				runes := utf8.RuneCountInString(body)
				assert.LessOrEqual(t, runes, maxBodyRunes, "%s: %s", dir, line)
			}
			switch {
			case pointer == "":
				forms["none"] = true
			case strings.Contains(pointer, "--rule"):
				forms["several"] = true
			default:
				forms["one"] = true
			}
		}
	}
	assert.Equal(t, map[string]bool{"none": true, "several": true, "one": true}, forms)
}
