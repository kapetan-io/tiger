package rules_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/driver/drivertest"
	"github.com/kapetan-io/tiger/internal/finish"
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

// emitted is one diagnostic a corpus replay produced, in the shape both
// replay paths share.
type emitted struct {
	category string
	message  string
}

// bound is a diagnostic resolved through the registry: the analyzer and
// rule ID its category binds to.
type bound struct {
	analyzer string
	ruleID   string
	message  string
}

// replay runs an analyzer's whole corpus through the driver its layout
// needs: analysistest over testdata/src for a per-package rule, the tiger
// driver's finish step over testdata/module for a whole-program rule
// (ADR-0010). Either way every emitted diagnostic comes back for the
// registry and style checks.
func replay(t *testing.T, analyzer *analysis.Analyzer, dirs []string) []emitted {
	t.Helper()
	collected := []emitted{}
	if rules.WholeProgram(analyzer.Name) {
		var finisher finish.Finisher
		for _, candidate := range rules.Finishers() {
			if candidate.Analyzer == analyzer {
				finisher = candidate
			}
		}
		require.NotNil(t, finisher.Run)
		root := filepath.Join(testdataPath(t, analyzer.Name), "module")
		for _, finding := range drivertest.Run(quietRun{}, root, finisher) {
			collected = append(collected, emitted{
				category: finding.Category, message: finding.Message,
			})
		}
		return collected
	}
	results := analysistest.Run(quietRun{}, testdataPath(t, analyzer.Name), analyzer, dirs...)
	for _, result := range results {
		for _, diagnostic := range result.Diagnostics {
			collected = append(collected, emitted{
				category: diagnostic.Category, message: diagnostic.Message,
			})
		}
	}
	return collected
}

// resolve maps an emitted diagnostic to the analyzer and rule ID the
// registry binds its category to, through the rules table or the facts
// table.
func resolve(diagnostic emitted) (bound, bool) {
	if rule, known := rules.ByCategory(diagnostic.category); known {
		return bound{
			analyzer: rule.Analyzer.Name, ruleID: rule.RuleID, message: diagnostic.message,
		}, true
	}
	if fact, known := rules.ByFact(diagnostic.category); known {
		return bound{
			analyzer: fact.Analyzer.Name, ruleID: fact.RuleID, message: diagnostic.message,
		}, true
	}
	return bound{}, false
}

// corpusFiles lists the source files a corpus is judged by: the rule-id
// directory for a per-package rule, every Go file in the module for a
// whole-program rule.
func corpusFiles(t *testing.T, corpus corpusFor) map[string]string {
	t.Helper()
	files := map[string]string{}
	if rules.WholeProgram(corpus.analyzerName) {
		root := filepath.Join(testdataPath(t, corpus.analyzerName), "module")
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return walkErr
			}
			files[entry.Name()] = readCorpusFile(t, path)
			return nil
		})
		require.NoError(t, err)
		return files
	}
	root := filepath.Join(testdataPath(t, corpus.analyzerName), "src", corpus.dir)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	for _, entry := range entries {
		files[entry.Name()] = readCorpusFile(t, filepath.Join(root, entry.Name()))
	}
	return files
}

// TestEveryRegisteredRuleHasCorpus enforces correctness constraint 3
// mechanically: every registered analyzer has, per rule it enforces, a
// failure-mode case that fires and a compliant rewrite that stays silent.
// Known misses are optional but must be marked when present. A
// whole-program rule's corpus is a module under testdata/module instead of
// a rule-id directory under testdata/src; the file-name convention is the
// same.
//
// Goal: a rule cannot be registered without its executable specification —
// an analyzer without its corpus does not merge.
func TestEveryRegisteredRuleHasCorpus(t *testing.T) {
	for _, corpus := range corpora(t) {
		t.Run(corpus.analyzerName+"/"+corpus.dir, func(t *testing.T) {
			// In the module layout the finding lands on the declaration
			// (the invariant const, the interface), which may sit in a
			// different package from the failure-shaped code that leaves it
			// undefended; the module as a whole must carry the expectation.
			wholeProgram := rules.WholeProgram(corpus.analyzerName)
			failures, compliants, wants := 0, 0, 0
			files := corpusFiles(t, corpus)
			for _, name := range slices.Sorted(maps.Keys(files)) {
				source := files[name]
				if strings.Contains(source, "want") {
					wants++
				}
				switch {
				case strings.HasPrefix(name, "failure"):
					failures++
					if !wholeProgram {
						assert.Contains(
							t,
							source,
							"want",
							"failure case %s must assert a firing diagnostic",
							name,
						)
					}
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
			assert.Positive(t, wants, "rule %s's corpus asserts no firing diagnostic",
				corpus.ruleID)
		})
	}
}

// TestEveryCorpusDiagnosticResolvesInRegistry enforces correctness
// constraint 1: no orphan diagnostics. It replays every corpus through the
// driver its layout needs and checks each emitted diagnostic against the
// registry.
//
// Goal: every diagnostic carries a category registered to the analyzer that
// emitted it — as a rule or as a fact — and its message leads with the
// registered rule ID.
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
			replayed := replay(t, analyzer, dirs)
			for _, diagnostic := range replayed {
				resolved, known := resolve(diagnostic)
				require.True(t, known,
					"diagnostic category %q is not in the registry", diagnostic.category)
				assert.Equal(t, analyzer.Name, resolved.analyzer)
				assert.True(t, strings.HasPrefix(resolved.message, resolved.ruleID+":"),
					"message %q must lead with %s:", resolved.message, resolved.ruleID)
			}
			assert.NotEmpty(t, replayed, "analyzer %s's corpus fired nothing", analyzer.Name)
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
	entries, err := os.ReadDir(filepath.Join("..", "analyzers"))
	require.NoError(t, err)
	packageDirs := map[string]bool{}
	for _, entry := range entries {
		packageDirs[entry.Name()] = entry.IsDir()
	}
	names := []string{}
	for _, analyzer := range rules.Analyzers() {
		names = append(names, analyzer.Name)
		assert.True(t, packageDirs[analyzer.Name],
			"analyzer %s has no package directory", analyzer.Name)
	}
	assert.Len(t, names, len(referenced))
	assert.IsIncreasing(t, names)

	// A whole-program rule's finish function is one per analyzer: every
	// entry of that analyzer that sets Finish sets the same one.
	finishOf := map[string]string{}
	for _, rule := range rules.CustomRules() {
		if rule.Finish == nil {
			continue
		}
		name := rule.Analyzer.Name
		id := fmt.Sprintf("%p", rule.Finish)
		if seen, found := finishOf[name]; found {
			assert.Equal(t, seen, id, "analyzer %s registers two finish functions", name)
		}
		finishOf[name] = id
	}
	assert.Len(t, rules.Finishers(), len(finishOf))
}

// TestFactsTableIsCoherent checks the computed-facts channel against the
// rules table.
//
// Goal: a fact category is never also a rule category (ADR-0012: facts
// are not rules and carry no severity), every fact's analyzer is
// registered, and its RuleID names a registered rule whose pin it feeds.
func TestFactsTableIsCoherent(t *testing.T) {
	ruleIDs := map[string]bool{}
	for _, id := range rules.RuleIDs() {
		ruleIDs[id] = true
	}
	analyzers := map[string]bool{}
	for _, analyzer := range rules.Analyzers() {
		analyzers[analyzer.Name] = true
	}
	categories := map[string]bool{}
	for _, fact := range rules.Facts() {
		_, isRule := rules.ByCategory(fact.Category)
		assert.False(t, isRule, "fact category %s is also a rule", fact.Category)
		assert.False(t, categories[fact.Category], "fact %s registered twice", fact.Category)
		categories[fact.Category] = true
		assert.True(t, strings.HasSuffix(fact.Category, "-facts"),
			"fact category %s must end in -facts", fact.Category)
		assert.True(t, ruleIDs[fact.RuleID], "fact %s feeds unregistered rule %s",
			fact.Category, fact.RuleID)
		assert.True(t, analyzers[fact.Analyzer.Name])
		assert.NotEmpty(t, fact.Title)
	}
}

// bannedWords is the vocabulary a message body may not use: tiger-internal
// terms a reader new to the dialect has never met. The specification's
// Diagnostics section is this list's human-readable twin; the two change
// together.
var bannedWords = []string{
	"pin", "pinned", "computed", "fact", "facts", "effect set", "frame condition",
	"lattice", "closed set", "back edge", "dominating", "synthesized", "ranking",
	"allowlist",
}

const (
	maxBodyRunes    = 240
	directivePrefix = "//tiger:"
)

var (
	ruleCodePattern = regexp.MustCompile(`TS-[A-Z]+[0-9]+`)
	directiveVerbs  = regexp.MustCompile(
		`\b(effects|frame|variant|requires|ensures|batched|restrict)\b`)
)

// bannedPattern matches word as a whole word, case-insensitively.
func bannedPattern(word string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
}

// checkStyle applies invariants I1–I6 of the diagnostic style guide to one
// diagnostic against the analyzer and rule it resolved to, naming the
// analyzer, the message, and the invariant in every failure.
func checkStyle(t *testing.T, resolved bound) {
	t.Helper()
	analyzerName, ruleID, message := resolved.analyzer, resolved.ruleID, resolved.message
	body, found := strings.CutPrefix(message, ruleID+": ")
	require.True(t, found, "I1: %s: %q must lead with %s: ", analyzerName, message, ruleID)
	assert.NotRegexp(t, ruleCodePattern, body,
		"I2: %s: body of %q repeats a rule code", analyzerName, message)
	assert.NotContains(t, message, "\n",
		"I3: %s: %q spans more than one line", analyzerName, message)
	assert.LessOrEqual(t, utf8.RuneCountInString(body), maxBodyRunes,
		"I4: %s: body of %q is %d characters, over the cap of %d",
		analyzerName, message, utf8.RuneCountInString(body), maxBodyRunes)
	for _, word := range bannedWords {
		assert.NotRegexp(t, bannedPattern(word), body,
			"I5: %s: body of %q uses the banned word %q", analyzerName, message, word)
	}
	if directiveVerbs.MatchString(body) {
		assert.Contains(t, body, directivePrefix,
			"I6: %s: body of %q names a directive without spelling %s",
			analyzerName, message, directivePrefix)
	}
}

// TestEveryCorpusMessageFollowsTheStyleGuide enforces the diagnostic
// style guide's invariants I1–I6 on every message every corpus emits: the
// body carries no second rule code, no newline, at most 240 characters, no
// banned internal vocabulary, and any directive it names is spelled
// //tiger:.
//
// Goal: a message a reader new to tiger cannot act on does not merge.
func TestEveryCorpusMessageFollowsTheStyleGuide(t *testing.T) {
	byAnalyzer := map[string][]corpusFor{}
	for _, corpus := range corpora(t) {
		byAnalyzer[corpus.analyzerName] = append(byAnalyzer[corpus.analyzerName], corpus)
	}
	for _, analyzer := range rules.Analyzers() {
		t.Run(analyzer.Name, func(t *testing.T) {
			dirs := []string{}
			for _, corpus := range byAnalyzer[analyzer.Name] {
				dirs = append(dirs, corpus.dir)
			}
			for _, diagnostic := range replay(t, analyzer, dirs) {
				resolved, known := resolve(diagnostic)
				require.True(t, known)
				checkStyle(t, resolved)
			}
		})
	}
}

// readCorpusFile reads one corpus source file.
func readCorpusFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}
