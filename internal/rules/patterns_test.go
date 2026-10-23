package rules_test

import (
	"bytes"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/pattern"
	"github.com/kapetan-io/tiger/internal/rules"
	"github.com/kapetan-io/tiger/patterns"
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

// registeredCodes is every rule code in the registry, custom and auto.
func registeredCodes() map[string]bool {
	codes := map[string]bool{}
	for _, rule := range rules.CustomRules() {
		codes[rule.RuleID] = true
	}
	for _, rule := range rules.AutoRules() {
		codes[rule.RuleID] = true
	}
	return codes
}

// TestRulePointersNamePatternsThatExist verifies the registry's Patterns
// field against the embedded collection.
//
// Goal: a finding never tells the reader to run `tiger pattern <name>` for a
// pattern that does not exist (ADR-0025).
func TestRulePointersNamePatternsThatExist(t *testing.T) {
	all, err := pattern.Load(patterns.Collection)
	require.NoError(t, err)
	for _, rule := range rules.CustomRules() {
		for _, name := range rule.Patterns {
			_, found := pattern.Find(all, name)
			assert.True(t, found, "%s names the missing pattern %s", rule.Category, name)
		}
	}
}

// TestPatternCardsNameRulesThatExist verifies every rule code on a card.
//
// Goal: a card never claims to back a rule the registry does not have.
func TestPatternCardsNameRulesThatExist(t *testing.T) {
	all, err := pattern.Load(patterns.Collection)
	require.NoError(t, err)
	codes := registeredCodes()
	for _, each := range all {
		for _, code := range each.Rules {
			assert.True(t, codes[code], "pattern %s backs the missing rule %s", each.Name, code)
		}
	}
}

// TestPatternsAndRulePointersAgree verifies both directions of the link.
//
// Goal: a pattern that backs a rule is named by that rule's registry entry,
// and a registry entry that names a pattern is on that pattern's card, so
// the pointer and the card change together.
func TestPatternsAndRulePointersAgree(t *testing.T) {
	all, err := pattern.Load(patterns.Collection)
	require.NoError(t, err)
	named := map[string][]string{}
	for _, rule := range rules.CustomRules() {
		for _, name := range rule.Patterns {
			named[name] = append(named[name], rule.RuleID)
		}
	}
	for _, each := range all {
		for _, code := range each.Rules {
			if !isCustom(code) {
				continue
			}
			assert.Contains(t, named[each.Name], code,
				"pattern %s backs %s but no %s entry names it", each.Name, code, code)
		}
		for _, code := range named[each.Name] {
			assert.Contains(t, each.Rules, code,
				"%s names pattern %s but its card does not list %s", code, each.Name, code)
		}
	}
}

func isCustom(code string) bool {
	return slices.Contains(rules.RuleIDs(), code)
}

// TestPatternChecklistsAskQuestions verifies each card gives reviewers
// checks, not statements.
//
// Goal: every pattern has at least three yes-or-no checklist questions.
func TestPatternChecklistsAskQuestions(t *testing.T) {
	all, err := pattern.Load(patterns.Collection)
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for _, each := range all {
		assert.GreaterOrEqual(t, len(each.Checklist), 3, each.Name)
		assert.False(t, strings.Contains(each.Name, " "), each.Name)
	}
}

// TestPointerEndsWithTheCommandsToRun verifies the suffix the drivers append.
//
// Goal: a category with one pattern points at it, one with several points at
// the list for its rule, and a category with none gets no suffix.
func TestPointerEndsWithTheCommandsToRun(t *testing.T) {
	for _, test := range []struct {
		name     string
		category string
		want     string
	}{
		{name: "OnePattern", category: "TS-S03", want: " — see `tiger pattern shutdown-loop`"},
		{name: "SeveralPatterns", category: "TS-C02", want: " — see `tiger pattern --rule TS-C02`"},
		{name: "NoPattern", category: "TS-S09", want: ""},
		{name: "ComputedFact", category: "TS-V01-facts", want: ""},
		{name: "Unregistered", category: "TS-Z99", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, rules.Pointer(test.category))
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
			require.Equal(t, command.ExitClean, got.code)
			listed := map[string]bool{}
			for _, line := range strings.Split(strings.TrimSpace(got.stdout), "\n") {
				name, _, _ := strings.Cut(line, "\t")
				listed[name] = true
			}
			assert.Equal(t, names, listed)
		})
	}
}
