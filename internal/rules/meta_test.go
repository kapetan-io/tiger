package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/rules"
)

// corpusFor pairs one analyzer with one rule's corpus directory.
type corpusFor struct {
	analyzerName string
	ruleID       string
	dir          string
}

// quietRun swallows analysistest's expectation mismatches; the meta-tests
// only consume the collected diagnostics, and each analyzer's own corpus
// test owns the expectation matching.
type quietRun struct{}

func (quietRun) Errorf(format string, args ...any) {}

// corpora derives the corpus list from the registry: one per distinct
// (analyzer, rule) pair.
func corpora(t *testing.T) []corpusFor {
	t.Helper()
	seen := map[string]bool{}
	listed := []corpusFor{}
	for _, rule := range rules.CustomRules() {
		key := rule.Analyzer.Name + "/" + rule.RuleID
		if seen[key] {
			continue
		}
		seen[key] = true
		listed = append(listed, corpusFor{
			analyzerName: rule.Analyzer.Name,
			ruleID:       rule.RuleID,
			dir:          rules.CorpusDir(rule.RuleID),
		})
	}
	return listed
}

// testdataPath locates an analyzer package's testdata directory relative to
// this package.
func testdataPath(t *testing.T, analyzerName string) string {
	t.Helper()
	absolute, err := filepath.Abs(filepath.Join("..", "analyzers", analyzerName, "testdata"))
	require.NoError(t, err)
	return absolute
}

// TestEveryRegisteredRuleHasCorpus enforces correctness constraint 3
// mechanically: every registered analyzer has, per rule it enforces, a
// failure-mode case that fires and a compliant rewrite that stays silent.
// Known misses are optional but must be marked when present.
//
// Goal: a rule cannot be registered without its executable specification —
// an analyzer without its corpus does not merge.
func TestEveryRegisteredRuleHasCorpus(t *testing.T) {
	for _, corpus := range corpora(t) {
		t.Run(corpus.analyzerName+"/"+corpus.dir, func(t *testing.T) {
			root := filepath.Join(testdataPath(t, corpus.analyzerName), "src", corpus.dir)
			entries, err := os.ReadDir(root)
			require.NoError(t, err)

			failures, compliants := 0, 0
			for _, entry := range entries {
				name := entry.Name()
				source := readCorpusFile(t, filepath.Join(root, name))
				switch {
				case strings.HasPrefix(name, "failure"):
					failures++
					assert.Contains(
						t,
						source,
						"want",
						"failure case %s must assert a firing diagnostic",
						name,
					)
				case strings.HasPrefix(name, "compliant"):
					compliants++
				case strings.HasPrefix(name, "knownmiss"):
					assert.Contains(t, source, "known-miss:",
						"known-miss case %s must say what coverage gap it documents", name)
					assert.NotContains(t, source, "// want",
						"known-miss case %s documents a silent gap, so nothing may fire", name)
				}
			}
			assert.Positive(
				t,
				failures,
				"rule %s needs a failure-mode case that fires",
				corpus.ruleID,
			)
			assert.Positive(
				t,
				compliants,
				"rule %s needs a compliant rewrite that stays silent",
				corpus.ruleID,
			)
		})
	}
}

// TestEveryCorpusDiagnosticResolvesInRegistry enforces correctness
// constraint 1: no orphan diagnostics. It replays every corpus through the
// analysistest driver and checks each emitted diagnostic against the
// registry.
//
// Goal: every diagnostic carries a category registered to the analyzer that
// emitted it, and its message leads with the registered rule ID.
func TestEveryCorpusDiagnosticResolvesInRegistry(t *testing.T) {
	byAnalyzer := map[string][]corpusFor{}
	for _, corpus := range corpora(t) {
		byAnalyzer[corpus.analyzerName] = append(byAnalyzer[corpus.analyzerName], corpus)
	}
	for _, analyzer := range rules.Analyzers() {
		t.Run(analyzer.Name, func(t *testing.T) {
			owned := byAnalyzer[analyzer.Name]
			require.NotEmpty(t, owned)
			dirs := make([]string, 0, len(owned))
			for _, corpus := range owned {
				dirs = append(dirs, corpus.dir)
			}
			results := analysistest.Run(
				quietRun{},
				testdataPath(t, analyzer.Name),
				analyzer,
				dirs...)
			emitted := 0
			for _, result := range results {
				for _, diagnostic := range result.Diagnostics {
					emitted++
					entry, known := rules.ByCategory(diagnostic.Category)
					require.True(t, known,
						"diagnostic category %q is not in the registry", diagnostic.Category)
					assert.Equal(t, analyzer.Name, entry.Analyzer.Name)
					assert.True(t, strings.HasPrefix(diagnostic.Message, entry.RuleID+":"),
						"message %q must lead with %s:", diagnostic.Message, entry.RuleID)
				}
			}
			assert.Positive(t, emitted, "analyzer %s's corpus fired nothing", analyzer.Name)
		})
	}
}

// TestRegistryIsCoherent checks the registry's own invariants.
//
// Goal: categories are unique, every entry names a TS rule and a live
// analyzer, and the analyzer set is exactly the set the entries reference.
func TestRegistryIsCoherent(t *testing.T) {
	categories := map[string]bool{}
	referenced := map[string]bool{}
	for _, rule := range rules.CustomRules() {
		assert.False(t, categories[rule.Category], "category %s registered twice", rule.Category)
		categories[rule.Category] = true
		assert.True(t, strings.HasPrefix(rule.RuleID, "TS-"))
		assert.True(t, strings.HasPrefix(rule.Category, rule.RuleID),
			"category %s must extend its rule ID %s", rule.Category, rule.RuleID)
		require.NotNil(t, rule.Analyzer)
		referenced[rule.Analyzer.Name] = true
		assert.NotEmpty(t, rule.Title)
	}
	names := []string{}
	for _, analyzer := range rules.Analyzers() {
		names = append(names, analyzer.Name)
		directory, err := os.Stat(filepath.Join("..", "analyzers", analyzer.Name))
		require.NoError(t, err)
		assert.True(t, directory.IsDir())
	}
	assert.Len(t, names, len(referenced))
	assert.IsIncreasing(t, names)
}

// readCorpusFile reads one corpus source file.
func readCorpusFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}
