// Package rules is the registry: the closed set of every rule,
// and the single source of rule identity.
//
// Custom rules bind a diagnostic category to the analyzer that enforces it
// and its severity. Auto rules live in internal/rules/auto. The tiger
// binary's analyzer set, the corpus meta-tests and the run's severity
// behavior are all derived from this package, so a rule cannot exist
// half-way: registered means enforced.
package rules

import (
	"maps"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/boundedloop"
	"github.com/kapetan-io/tiger/internal/analyzers/chandecl"
	"github.com/kapetan-io/tiger/internal/analyzers/comments"
	"github.com/kapetan-io/tiger/internal/analyzers/compoundcond"
	"github.com/kapetan-io/tiger/internal/analyzers/contracts"
	"github.com/kapetan-io/tiger/internal/analyzers/declorder"
	"github.com/kapetan-io/tiger/internal/analyzers/deferdistance"
	"github.com/kapetan-io/tiger/internal/analyzers/directives"
	"github.com/kapetan-io/tiger/internal/analyzers/errignore"
	"github.com/kapetan-io/tiger/internal/analyzers/invariantnegative"
	"github.com/kapetan-io/tiger/internal/analyzers/invariantrefs"
	"github.com/kapetan-io/tiger/internal/analyzers/ioinloop"
	"github.com/kapetan-io/tiger/internal/analyzers/layers"
	"github.com/kapetan-io/tiger/internal/analyzers/limitflow"
	"github.com/kapetan-io/tiger/internal/analyzers/limitrelate"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder"
	"github.com/kapetan-io/tiger/internal/analyzers/nogoroutine"
	"github.com/kapetan-io/tiger/internal/analyzers/nogoto"
	"github.com/kapetan-io/tiger/internal/analyzers/norecursion"
	"github.com/kapetan-io/tiger/internal/analyzers/paniccheck"
	"github.com/kapetan-io/tiger/internal/analyzers/participle"
	"github.com/kapetan-io/tiger/internal/analyzers/poolzero"
	"github.com/kapetan-io/tiger/internal/analyzers/returnarity"
	"github.com/kapetan-io/tiger/internal/analyzers/sametypeparams"
	"github.com/kapetan-io/tiger/internal/analyzers/selectctx"
	"github.com/kapetan-io/tiger/internal/analyzers/skipcheck"
	"github.com/kapetan-io/tiger/internal/analyzers/tablename"
	"github.com/kapetan-io/tiger/internal/analyzers/testpackage"
	"github.com/kapetan-io/tiger/internal/analyzers/variant"
	"github.com/kapetan-io/tiger/internal/finish"
)

type Severity int

const (
	SeverityBlocking Severity = iota
	SeverityAdvisory
)

type Fact struct {
	Category string
	RuleID   string
	Analyzer *analysis.Analyzer
	Title    string
}

type CustomRule struct {
	Category string
	RuleID   string
	Analyzer *analysis.Analyzer
	Severity Severity
	Title    string
	Finish   finish.Func
	Counted  string
	Patterns []string
}

func (r CustomRule) CountedNoun(n int) string {
	if n == 1 {
		return strings.TrimSuffix(r.Counted, "s")
	}
	return r.Counted
}

var customRules = []CustomRule{
	{
		Category: "TS-S09", RuleID: "TS-S09", Analyzer: nogoto.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no goto, no labeled break or continue",
	},
	{
		Category: "TS-S18", RuleID: "TS-S18", Analyzer: paniccheck.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no naked panic outside the assert package",
	},
	{
		Category: "TS-S02", RuleID: "TS-S02", Analyzer: boundedloop.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"capped-worklist", "filtered-page", "stream-page"},
		Title:    "every loop has an upper bound",
	},
	{
		Category: "TS-S02-limit", RuleID: "TS-S02", Analyzer: limitflow.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"clamped-limit"},
		Title:    "every loop limit has an upper bound where it enters the program",
	},
	{
		Category: "TS-S03", RuleID: "TS-S03", Analyzer: boundedloop.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"shutdown-loop"},
		Title:    "an unbounded event loop selects on a stop case",
	},
	{
		Category: "TS-S23", RuleID: "TS-S23", Analyzer: boundedloop.Analyzer,
		Severity: SeverityAdvisory,
		Patterns: []string{"shutdown-loop"},
		Title:    "an event loop's only exit is its stop case",
		Counted:  "extra event loop exits",
	},
	{
		Category: "TS-S06", RuleID: "TS-S06", Analyzer: compoundcond.Analyzer,
		Severity: SeverityBlocking,
		Title:    "one logical operator per condition",
	},
	{
		Category: "TS-S07", RuleID: "TS-S07", Analyzer: compoundcond.Analyzer,
		Severity: SeverityBlocking,
		Title:    "split compound assertions",
	},
	{
		Category: "TS-S08", RuleID: "TS-S08", Analyzer: compoundcond.Analyzer,
		Severity: SeverityBlocking,
		Title:    "a switch over a closed set ends in assert.Unreachable",
	},
	{
		Category: "TS-C02", RuleID: "TS-C02", Analyzer: nogoroutine.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"goroutine-owner", "shutdown-loop"},
		Title:    "every goroutine starts through a group that waits for it",
	},
	{
		Category: "TS-C03", RuleID: "TS-C03", Analyzer: nogoroutine.Analyzer,
		Severity: SeverityBlocking,
		Title:    "a package that starts goroutines checks for leaks in its tests",
	},
	{
		Category: "TS-C09", RuleID: "TS-C09", Analyzer: nogoroutine.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no spawning work in direct reaction to an external event",
	},
	{
		Category: "TS-C05", RuleID: "TS-C05", Analyzer: selectctx.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"shutdown-loop"},
		Title:    "every blocking receive or send has a stop case",
	},
	{
		Category: "TS-C12", RuleID: "TS-C12", Analyzer: chandecl.Analyzer,
		Severity: SeverityBlocking,
		Title:    "channel types are declared in one file per package",
	},
	{
		Category: "TS-E02", RuleID: "TS-E02", Analyzer: errignore.Analyzer,
		Severity: SeverityBlocking,
		Title:    "an error discarded with _ is a finding, whatever comment sits next to it",
	},
	{
		Category: "TS-E06", RuleID: "TS-E06", Analyzer: returnarity.Analyzer,
		Severity: SeverityBlocking,
		Title:    "minimize return arity",
	},
	{
		Category: "TS-L09", RuleID: "TS-L09", Analyzer: directives.Analyzer,
		Severity: SeverityBlocking,
		Title:    "every directive parses and every escape carries a reason",
	},
	{
		Category: "TS-L09-escape", RuleID: "TS-L09", Analyzer: directives.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "every escape directive is counted against the package budget",
		Counted:  "escape directives",
	},
	{
		Category: "TS-D07", RuleID: "TS-D07", Analyzer: skipcheck.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "skipped tests are counted against the package budget",
		Counted:  "skipped tests",
	},
	{
		Category: "TS-D06", RuleID: "TS-D06",
		Severity: SeverityBlocking,
		Title:    "per-package budgets may only decrease",
	},
	{
		Category: "TS-T10", RuleID: "TS-T10", Analyzer: tablename.Analyzer,
		Severity: SeverityBlocking,
		Title:    "table-driven tests have named cases",
	},
	{
		Category: "TS-L14", RuleID: "TS-L14", Analyzer: comments.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no prose comments below the package line of a production file",
	},
	{
		Category: "TS-T12", RuleID: "TS-T12", Analyzer: testpackage.Analyzer,
		Severity: SeverityBlocking,
		Title:    "tests live in the external _test package",
	},
	{
		Category: "TS-S21", RuleID: "TS-S21", Analyzer: limitrelate.Analyzer,
		Severity: SeverityBlocking,
		Title:    "every limit constant participates in a relational assertion",
	},
	{
		Category: "TS-N07", RuleID: "TS-N07", Analyzer: sametypeparams.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no adjacent same-type parameters, at most four parameters",
	},
	{
		Category: "TS-N08", RuleID: "TS-N08", Analyzer: sametypeparams.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no bool parameters; use a named type",
	},
	{
		Category: "TS-N14", RuleID: "TS-N14", Analyzer: participle.Analyzer,
		Severity: SeverityBlocking,
		Title:    "exported identifiers do not end in a present participle",
	},
	{
		Category: "TS-L10", RuleID: "TS-L10", Analyzer: deferdistance.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no defer inside a loop",
	},
	{
		Category: "TS-L10-distance", RuleID: "TS-L10", Analyzer: deferdistance.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "a defer stays next to its acquisition",
		Counted:  "defers away from their acquisitions",
	},
	{
		Category: "TS-M10", RuleID: "TS-M10", Analyzer: ioinloop.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no IO inside a loop body",
	},
	{
		Category: "TS-L05", RuleID: "TS-L05", Analyzer: declorder.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "struct order is fields, nested types, constructor, methods",
		Counted:  "misordered struct declarations",
	},
	{
		Category: "TS-S01", RuleID: "TS-S01", Analyzer: norecursion.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"capped-worklist"},
		Title:    "no recursion; cycles in the static call graph are findings",
	},
	{
		Category: "TS-T02", RuleID: "TS-T02", Analyzer: maporder.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"sorted-map-output"},
		Title:    "map iteration order never reaches an output",
	},
	{
		Category: "TS-M05", RuleID: "TS-M05", Analyzer: poolzero.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"pool-reuse"},
		Title:    "Put is preceded by zeroing the value",
	},
	{
		Category: "TS-V01", RuleID: "TS-V01", Analyzer: variant.Analyzer,
		Severity: SeverityBlocking,
		Title:    "every unbounded loop has a verified variant",
	},
	{
		Category: "TS-V03", RuleID: "TS-V03", Analyzer: contracts.Analyzer,
		Severity: SeverityBlocking,
		Title:    "preconditions are declared and discharged at call sites",
	},
	{
		Category: "TS-A07", RuleID: "TS-A07", Analyzer: invariantrefs.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"ledger"},
		Title:    "every invariant is asserted in a function outside _test.go files",
		Finish:   invariantrefs.Finish,
	},
	{
		Category: "TS-A09", RuleID: "TS-A09", Analyzer: invariantnegative.Analyzer,
		Severity: SeverityBlocking,
		Patterns: []string{"ledger"},
		Title:    "every invariant has a test that violates it",
		Finish:   invariantnegative.Finish,
	},
	{
		Category: "TS-X03", RuleID: "TS-X03", Analyzer: layers.Analyzer,
		Severity: SeverityBlocking,
		Title:    "the package import graph is declared and enforced",
	},
}

var facts = []Fact{
	{
		Category: "TS-V01-facts", RuleID: "TS-V01", Analyzer: variant.Analyzer,
		Title: "synthesized variants print in pin syntax under --show-facts",
	},
}

func Facts() []Fact {
	listed := make([]Fact, len(facts))
	copy(listed, facts)
	return listed
}

func ByFact(category string) (Fact, bool) {
	for _, fact := range facts {
		if fact.Category == category {
			return fact, true
		}
	}
	return Fact{}, false
}

func CustomRules() []CustomRule {
	listed := make([]CustomRule, len(customRules))
	copy(listed, customRules)
	return listed
}

func ByCategory(category string) (CustomRule, bool) {
	for _, rule := range customRules {
		if rule.Category == category {
			return rule, true
		}
	}
	return CustomRule{}, false
}

func Analyzers() []*analysis.Analyzer {
	byName := map[string]*analysis.Analyzer{}
	for _, rule := range customRules {
		if rule.Analyzer == nil {
			continue
		}
		byName[rule.Analyzer.Name] = rule.Analyzer
	}
	names := slices.Sorted(maps.Keys(byName))
	listed := make([]*analysis.Analyzer, 0, len(names))
	for _, name := range names {
		listed = append(listed, byName[name])
	}
	return listed
}

func Finishers() []finish.Finisher {
	byName := map[string]finish.Finisher{}
	for _, rule := range customRules {
		if rule.Finish == nil {
			continue
		}
		if _, seen := byName[rule.Analyzer.Name]; seen {
			continue
		}
		byName[rule.Analyzer.Name] = finish.Finisher{Analyzer: rule.Analyzer, Run: rule.Finish}
	}
	listed := make([]finish.Finisher, 0, len(byName))
	for _, name := range slices.Sorted(maps.Keys(byName)) {
		listed = append(listed, byName[name])
	}
	return listed
}

func WholeProgram(analyzerName string) bool {
	for _, rule := range customRules {
		if rule.Analyzer != nil && rule.Analyzer.Name == analyzerName && rule.Finish != nil {
			return true
		}
	}
	return false
}

func RuleIDs() []string {
	seen := map[string]bool{}
	ids := make([]string, 0, len(customRules))
	for _, rule := range customRules {
		if rule.Analyzer != nil && !seen[rule.RuleID] {
			seen[rule.RuleID] = true
			ids = append(ids, rule.RuleID)
		}
	}
	sort.Strings(ids)
	return ids
}

func CountedCodes() map[string]bool {
	codes := map[string]bool{}
	for _, rule := range customRules {
		if rule.Severity == SeverityAdvisory {
			codes[rule.RuleID] = true
		}
	}
	return codes
}

func CorpusDir(ruleID string) string {
	return strings.ToLower(ruleID)
}

func Pointer(category string) string {
	rule, known := ByCategory(category)
	switch {
	case !known || len(rule.Patterns) == 0:
		return ""
	case len(rule.Patterns) == 1:
		return " — see `tiger pattern " + rule.Patterns[0] + "`"
	}
	return " — see `tiger pattern --rule " + rule.RuleID + "`"
}
