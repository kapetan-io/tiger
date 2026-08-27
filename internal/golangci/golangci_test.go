package golangci_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/golangci"
)

// bannedWords is the vocabulary a finding body may not use: tiger-internal
// terms a reader new to the dialect has never met. It mirrors the list in
// the specification's Diagnostics section and internal/rules/meta_test.go.
var bannedWords = []string{
	"pin", "pinned", "computed", "fact", "facts", "effect set", "frame condition",
	"lattice", "closed set", "back edge", "dominating", "synthesized", "ranking",
	"allowlist",
}

var (
	ruleCodePattern  = regexp.MustCompile(`TS-[A-Z]+[0-9]+`)
	directiveVerbs   = regexp.MustCompile(`\b(effects|frame|variant|requires|ensures|batched)\b`)
	maxBodyRunes     = 240
	directivePrefix  = "//tiger:"
	bannedWordChecks = compileBanned()
)

func compileBanned() map[string]*regexp.Regexp {
	compiled := map[string]*regexp.Regexp{}
	for _, word := range bannedWords {
		compiled[word] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	}
	return compiled
}

// checkStyle applies invariants I1–I6 of the diagnostic style guide to one
// audit finding, naming the invariant in every failure.
func checkStyle(t *testing.T, finding golangci.Finding) {
	t.Helper()
	ruleID, message := finding.RuleID, finding.Message
	body, found := strings.CutPrefix(message, ruleID+": ")
	require.True(t, found, "I1: %q must lead with %s: ", message, ruleID)
	assert.NotRegexp(t, ruleCodePattern, body, "I2: body of %q repeats a rule code", message)
	assert.NotContains(t, message, "\n", "I3: %q spans more than one line", message)
	assert.LessOrEqual(t, utf8.RuneCountInString(body), maxBodyRunes,
		"I4: body of %q is %d characters, over the cap", message, utf8.RuneCountInString(body))
	for _, word := range bannedWords {
		assert.NotRegexp(t, bannedWordChecks[word], body,
			"I5: body of %q uses the banned word %q", message, word)
	}
	if directiveVerbs.MatchString(body) {
		assert.Contains(t, body, directivePrefix,
			"I6: body of %q names a directive without spelling %s", message, directivePrefix)
	}
}

// TestVerifyFindingsFollowTheStyleGuide covers the audit channel's wording
// contract.
//
// Goal: every line tiger golangci emits over a config exhibiting all three
// finding shapes — a missing linter, an absent setting, a drifted setting
// — satisfies invariants I1–I6: one rule code in the prefix and none in
// the body, one line, at most 240 characters of body, no internal
// vocabulary, and directives spelled literally.
func TestVerifyFindingsFollowTheStyleGuide(t *testing.T) {
	findings, err := golangci.Verify(filepath.Join("testdata", "fixtures", "audit"))
	require.NoError(t, err)
	require.NotEmpty(t, findings)

	shapes := map[string]bool{}
	for _, finding := range findings {
		switch {
		case strings.Contains(finding.Message, "is missing from"):
			shapes["missing"] = true
		case strings.Contains(finding.Message, "isn't set"):
			shapes["absent"] = true
		case strings.Contains(finding.Message, "but tiger's baseline"):
			shapes["drifted"] = true
		}
		t.Run(finding.RuleID+"/"+finding.Linter, func(t *testing.T) {
			checkStyle(t, finding)
		})
	}
	assert.Len(t, shapes, 3, "fixture must exercise all three finding shapes, got %v", shapes)
}

// TestVerifyMissingLinterNamesTheEdit covers the missing-linter shape.
//
// Goal: the finding says which linter is missing, which rule that leaves
// unenforced, and the exact config edit that restores it.
func TestVerifyMissingLinterNamesTheEdit(t *testing.T) {
	findings, err := golangci.Verify(filepath.Join("testdata", "fixtures", "audit"))
	require.NoError(t, err)
	var got string
	for _, finding := range findings {
		if finding.Linter == "cyclop" && strings.Contains(finding.Message, "is missing from") {
			got = finding.Message
		}
	}
	assert.Equal(t, "TS-S05: cyclop is missing from linters.enable, so nothing enforces the "+
		"rule \"cyclomatic complexity at most 10\" — add cyclop to linters.enable", got)
}
