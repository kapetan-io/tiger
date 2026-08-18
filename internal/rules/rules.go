// Package rules is the registry: the closed set of every rule in the
// dialect, and the single source of rule identity.
//
// Custom rules bind a diagnostic category to the analyzer that enforces it
// and its severity; auto rules (auto.go) bind a rule to the golangci-lint
// linter and baseline settings that enforce it. The tiger binary's analyzer
// set, the corpus meta-tests, the run's severity behavior, and the
// tiger golangci audit are all derived from this package, so a rule cannot
// exist half-way: registered means enforced.
package rules

import (
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/boundedloop"
	"github.com/kapetan-io/tiger/internal/analyzers/chandecl"
	"github.com/kapetan-io/tiger/internal/analyzers/compoundcond"
	"github.com/kapetan-io/tiger/internal/analyzers/declorder"
	"github.com/kapetan-io/tiger/internal/analyzers/declusedistance"
	"github.com/kapetan-io/tiger/internal/analyzers/deferdistance"
	"github.com/kapetan-io/tiger/internal/analyzers/derivation"
	"github.com/kapetan-io/tiger/internal/analyzers/directives"
	"github.com/kapetan-io/tiger/internal/analyzers/errignore"
	"github.com/kapetan-io/tiger/internal/analyzers/ioinloop"
	"github.com/kapetan-io/tiger/internal/analyzers/limitrelate"
	"github.com/kapetan-io/tiger/internal/analyzers/nogoroutine"
	"github.com/kapetan-io/tiger/internal/analyzers/nogoto"
	"github.com/kapetan-io/tiger/internal/analyzers/paniccheck"
	"github.com/kapetan-io/tiger/internal/analyzers/participle"
	"github.com/kapetan-io/tiger/internal/analyzers/returnarity"
	"github.com/kapetan-io/tiger/internal/analyzers/sametypeparams"
	"github.com/kapetan-io/tiger/internal/analyzers/selectctx"
	"github.com/kapetan-io/tiger/internal/analyzers/skipcheck"
	"github.com/kapetan-io/tiger/internal/analyzers/tablename"
	"github.com/kapetan-io/tiger/internal/analyzers/testdoc"
)

// Severity is a rule's run-level consequence, defined once per rule here and
// never inside an analyzer (ADR-0002).
type Severity int

const (
	// SeverityBlocking findings fail the run: exit code 1.
	SeverityBlocking Severity = iota
	// SeverityAdvisory findings print, marked as advisory, and are counted;
	// they never affect the exit code.
	SeverityAdvisory
	// SeverityReported exists in the model for later waves
	// (annotation-only); no wave-1 rule uses it.
	SeverityReported
)

// CustomRule binds one diagnostic category to the rule it enforces, the
// analyzer that emits it, and its severity. A rule with split severity
// (TS-L09, TS-L10) registers each half as its own category at its own level.
type CustomRule struct {
	// Category is the analysis.Diagnostic category analyzers tag findings
	// with; the driver resolves severity through it.
	Category string
	// RuleID is the specification rule, TS-*. Every diagnostic message
	// starts with this ID.
	RuleID string
	// Analyzer is the pass that enforces the rule.
	Analyzer *analysis.Analyzer
	// Severity is the run-level consequence the driver applies.
	Severity Severity
	// Title is the rule, one short sentence.
	Title string
}

// customRules is the wave-1 dialect. Order groups rules by analyzer.
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
		Title:    "every loop has an upper bound",
	},
	{
		Category: "TS-S03", RuleID: "TS-S03", Analyzer: boundedloop.Analyzer,
		Severity: SeverityBlocking,
		Title:    "an unbounded event loop selects on ctx.Done()",
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
		Title:    "every goroutine starts through a supervisor",
	},
	{
		Category: "TS-C09", RuleID: "TS-C09", Analyzer: nogoroutine.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no spawning work in direct reaction to an external event",
	},
	{
		Category: "TS-C05", RuleID: "TS-C05", Analyzer: selectctx.Analyzer,
		Severity: SeverityBlocking,
		Title:    "every blocking receive or send has a ctx.Done() case",
	},
	{
		Category: "TS-C12", RuleID: "TS-C12", Analyzer: chandecl.Analyzer,
		Severity: SeverityBlocking,
		Title:    "channel types are declared in one file per package",
	},
	{
		Category: "TS-E02", RuleID: "TS-E02", Analyzer: errignore.Analyzer,
		Severity: SeverityBlocking,
		Title:    "discarding a result with _ requires a justification comment",
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
		Title:    "every escape directive surfaces on every run",
	},
	{
		Category: "TS-D07", RuleID: "TS-D07", Analyzer: skipcheck.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "skipped tests are reported on every run",
	},
	{
		Category: "TS-T10", RuleID: "TS-T10", Analyzer: tablename.Analyzer,
		Severity: SeverityBlocking,
		Title:    "table-driven tests have named cases",
	},
	{
		Category: "TS-T06", RuleID: "TS-T06", Analyzer: testdoc.Analyzer,
		Severity: SeverityBlocking,
		Title:    "test functions have a doc comment",
	},
	{
		Category: "TS-S22", RuleID: "TS-S22", Analyzer: derivation.Analyzer,
		Severity: SeverityBlocking,
		Title:    "a limit constant's stated derivation evaluates to its value",
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
	},
	{
		Category: "TS-M10", RuleID: "TS-M10", Analyzer: ioinloop.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no IO inside a loop body",
	},
	{
		Category: "TS-N06", RuleID: "TS-N06", Analyzer: declorder.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "a helper is prefixed with the name of its caller",
	},
	{
		Category: "TS-L05", RuleID: "TS-L05", Analyzer: declorder.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "struct order is fields, nested types, constructor, methods",
	},
	{
		Category: "TS-S13", RuleID: "TS-S13", Analyzer: declusedistance.Analyzer,
		Severity: SeverityAdvisory,
		Title:    "variables are declared at the point of first use",
	},
}

// CustomRules returns every registered custom-rule category.
func CustomRules() []CustomRule {
	listed := make([]CustomRule, len(customRules))
	copy(listed, customRules)
	return listed
}

// ByCategory resolves a diagnostic category to its registry entry. Every
// diagnostic a wave-1 analyzer emits must resolve here (constraint 1).
func ByCategory(category string) (CustomRule, bool) {
	for _, rule := range customRules {
		if rule.Category == category {
			return rule, true
		}
	}
	return CustomRule{}, false
}

// Analyzers returns every registered analyzer once, sorted by name. This is
// the only path into the tiger binary and the plugin: an analyzer that is
// not registered here cannot ship.
func Analyzers() []*analysis.Analyzer {
	byName := map[string]*analysis.Analyzer{}
	for _, rule := range customRules {
		byName[rule.Analyzer.Name] = rule.Analyzer
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	listed := make([]*analysis.Analyzer, 0, len(names))
	for _, name := range names {
		listed = append(listed, byName[name])
	}
	return listed
}

// RuleIDs returns the distinct custom rule IDs, sorted. The corpus meta-test
// walks these: every rule needs its failure-mode and compliant cases.
func RuleIDs() []string {
	seen := map[string]bool{}
	ids := make([]string, 0, len(customRules))
	for _, rule := range customRules {
		if !seen[rule.RuleID] {
			seen[rule.RuleID] = true
			ids = append(ids, rule.RuleID)
		}
	}
	sort.Strings(ids)
	return ids
}

// CorpusDir names the analysistest corpus package for a rule: the rule ID
// lowercased, for example ts-s09.
func CorpusDir(ruleID string) string {
	return strings.ToLower(ruleID)
}
