package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/rules"
)

const (
	referenceDoc = "Tiger Rule Reference.md"
	explainerDoc = "Tiger Explainer.md"
)

// blockingMarker and advisoryMarker are the severity lines every reference
// entry states; the reference and the registry must agree per category.
const (
	blockingMarker = "**Severity:** blocking"
	advisoryMarker = "**Severity:** advisory"
)

// docContent reads one documentation file from the repository docs directory.
func docContent(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "docs", name))
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// entryBlock returns the reference text between a rule category's anchor
// comment and the next anchor (or the end of the document).
func entryBlock(t *testing.T, reference string, rule rules.CustomRule) string {
	t.Helper()
	anchor := "<!-- rule: " + rule.Category + " -->"
	require.Equal(t, 1, strings.Count(reference, anchor))
	block := reference[strings.Index(reference, anchor)+len(anchor):]
	if next := strings.Index(block, "<!-- rule: "); next >= 0 {
		block = block[:next]
	}
	return block
}

// TestReferenceCoversEveryCustomRule verifies the rule reference carries one
// anchored entry per registered custom-rule category, and that each entry
// states the severity the registry defines for it.
func TestReferenceCoversEveryCustomRule(t *testing.T) {
	reference := docContent(t, referenceDoc)
	for _, rule := range rules.CustomRules() {
		block := entryBlock(t, reference, rule)
		want, other := blockingMarker, advisoryMarker
		if rule.Severity == rules.SeverityAdvisory {
			want, other = advisoryMarker, blockingMarker
		}
		assert.Contains(t, block, want)
		assert.NotContains(t, block, other)
	}
}

// TestReferenceCoversEveryAutoRule verifies every auto rule's ID and
// enforcing linter appear in the rule reference's auto-rule table.
func TestReferenceCoversEveryAutoRule(t *testing.T) {
	reference := docContent(t, referenceDoc)
	for _, rule := range rules.AutoRules() {
		assert.Contains(t, reference, rule.RuleID)
		assert.Contains(t, reference, rule.Linter)
	}
}

// TestDocsCarryNoRemovedVocabulary verifies neither end-user document names
// a rule removed from the dialect (ENG-161) or the retired output marker.
func TestDocsCarryNoRemovedVocabulary(t *testing.T) {
	for _, name := range []string{referenceDoc, explainerDoc} {
		content := docContent(t, name)
		for _, forbidden := range []string{"TS-N12", "TS-N13", "TS-N15", "[advisory]"} {
			assert.NotContains(t, content, forbidden)
		}
	}
}
