// Package rules is the registry: the closed set of every rule,
// and the single source of rule identity.
//
// Custom rules bind a diagnostic category to the analyzer that enforces it
// and its severity; auto rules (auto.go) bind a rule to the golangci-lint
// linter and baseline settings that enforce it. The tiger binary's analyzer
// set, the corpus meta-tests, the run's severity behavior, and the
// tiger golangci audit are all derived from this package, so a rule cannot
// exist half-way: registered means enforced.
package rules

import (
	"maps"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/boundedloop"
	"github.com/kapetan-io/tiger/internal/analyzers/chandecl"
	"github.com/kapetan-io/tiger/internal/analyzers/closedworld"
	"github.com/kapetan-io/tiger/internal/analyzers/compoundcond"
	"github.com/kapetan-io/tiger/internal/analyzers/contracts"
	"github.com/kapetan-io/tiger/internal/analyzers/declorder"
	"github.com/kapetan-io/tiger/internal/analyzers/deferdistance"
	"github.com/kapetan-io/tiger/internal/analyzers/derivation"
	"github.com/kapetan-io/tiger/internal/analyzers/directives"
	"github.com/kapetan-io/tiger/internal/analyzers/effects"
	"github.com/kapetan-io/tiger/internal/analyzers/errignore"
	"github.com/kapetan-io/tiger/internal/analyzers/frames"
	"github.com/kapetan-io/tiger/internal/analyzers/invariantnegative"
	"github.com/kapetan-io/tiger/internal/analyzers/invariantrefs"
	"github.com/kapetan-io/tiger/internal/analyzers/ioinloop"
	"github.com/kapetan-io/tiger/internal/analyzers/limitrelate"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder"
	"github.com/kapetan-io/tiger/internal/analyzers/nogoroutine"
	"github.com/kapetan-io/tiger/internal/analyzers/nogoto"
	"github.com/kapetan-io/tiger/internal/analyzers/norecursion"
	"github.com/kapetan-io/tiger/internal/analyzers/paniccheck"
	"github.com/kapetan-io/tiger/internal/analyzers/participle"
	"github.com/kapetan-io/tiger/internal/analyzers/poolzero"
	"github.com/kapetan-io/tiger/internal/analyzers/restrictions"
	"github.com/kapetan-io/tiger/internal/analyzers/returnarity"
	"github.com/kapetan-io/tiger/internal/analyzers/sametypeparams"
	"github.com/kapetan-io/tiger/internal/analyzers/selectctx"
	"github.com/kapetan-io/tiger/internal/analyzers/singleimpl"
	"github.com/kapetan-io/tiger/internal/analyzers/skipcheck"
	"github.com/kapetan-io/tiger/internal/analyzers/tablename"
	"github.com/kapetan-io/tiger/internal/analyzers/testdoc"
	"github.com/kapetan-io/tiger/internal/analyzers/variant"
	"github.com/kapetan-io/tiger/internal/finish"
)

// Severity is a rule's run-level consequence, defined once per rule here and
// never inside an analyzer (ADR-0002). There are exactly two: a rule either
// blocks or it is not a rule (ADR-0012). Computed facts — the --show-facts
// channel tiger pin freezes from — are not rules and carry no severity; they
// live in the separate Facts table.
type Severity int

const (
	// SeverityBlocking findings fail the run: exit code 1.
	SeverityBlocking Severity = iota
	// SeverityAdvisory findings are counted per package against
	// tiger.budget.yaml by the CLI driver: under budget they never print,
	// and on overrun they print as blocking lines under a TS-D06 line.
	// Reserved for the standing notices the specification defines
	// (escapes, skipped tests) and ADR-0006's advisory trial.
	SeverityAdvisory
)

// Fact binds one computed-fact category to the analyzer that emits it. A
// fact is not a rule: it is the current value of something a pin can
// freeze (an effect set, a frame, a loop variant), printed only under
// --show-facts, never counted, never a verdict. Its RuleID names the rule
// whose pin the fact feeds.
type Fact struct {
	Category string
	RuleID   string
	Analyzer *analysis.Analyzer
	Title    string
}

// CustomRule binds one diagnostic category to the rule it enforces, the
// analyzer that emits it, and its severity. A rule with split severity
// (TS-L09, TS-L10) registers each half as its own category at its own level.
// A driver-enforced rule (TS-D06) has no analyzer and no corpus: the CLI
// produces its lines itself.
type CustomRule struct {
	// Category is the analysis.Diagnostic category analyzers tag findings
	// with; the driver resolves severity through it.
	Category string
	// RuleID is the specification rule, TS-*. Every diagnostic message
	// starts with this ID.
	RuleID string
	// Analyzer is the pass that enforces the rule; nil for a rule the
	// driver enforces.
	Analyzer *analysis.Analyzer
	// Severity is the run-level consequence the driver applies.
	Severity Severity
	// Title is the rule, one short sentence.
	Title string
	// Finish is the whole-program half of a rule whose evidence is spread
	// across packages (ADR-0010): the driver calls it once after every
	// package has been visited. Nil for every per-package rule.
	Finish finish.Func
	// Counted is the plural noun phrase a TS-D06 line uses for an advisory
	// rule's findings ("skipped tests"); empty on every other severity.
	Counted string
}

// CountedNoun renders the rule's counted noun phrase for n findings:
// plural, or singular when n is 1.
func (r CustomRule) CountedNoun(n int) string {
	if n == 1 {
		return strings.TrimSuffix(r.Counted, "s")
	}
	return r.Counted
}

// customRules is the custom half of the rule set: the wave-1 rules, the SSA wave's, then the
// cross-package wave's. Order groups rules by analyzer.
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
		Title:    "no recursion; cycles in the static call graph are findings",
	},
	{
		Category: "TS-T02", RuleID: "TS-T02", Analyzer: maporder.Analyzer,
		Severity: SeverityBlocking,
		Title:    "map iteration order never reaches an output",
	},
	{
		Category: "TS-M05", RuleID: "TS-M05", Analyzer: poolzero.Analyzer,
		Severity: SeverityBlocking,
		Title:    "pooled types implement Reset, and Put is preceded by a reset",
	},
	{
		Category: "TS-F01", RuleID: "TS-F01", Analyzer: effects.Analyzer,
		Severity: SeverityBlocking,
		Title:    "an effects pin is an exact, bidirectional contract",
	},
	{
		Category: "TS-F02", RuleID: "TS-F02", Analyzer: effects.Analyzer,
		Severity: SeverityBlocking,
		Title:    "a pin bounds the entire subtree beneath it",
	},
	{
		Category: "TS-F07", RuleID: "TS-F07", Analyzer: frames.Analyzer,
		Severity: SeverityBlocking,
		Title:    "writes outside a pinned frame fail, bidirectionally",
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
		Category: "TS-P01", RuleID: "TS-P01", Analyzer: restrictions.Analyzer,
		Severity: SeverityBlocking,
		Title:    "declared package restrictions hold against the package's own imports",
	},
	{
		Category: "TS-P02", RuleID: "TS-P02", Analyzer: restrictions.Analyzer,
		Severity: SeverityBlocking,
		Title:    "every transitive dependency supports each restriction axis a package claims",
	},
	{
		Category: "TS-K03", RuleID: "TS-K03", Analyzer: closedworld.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no dynamic dispatch in a package that declares closed-dispatch",
	},
	{
		Category: "TS-A07", RuleID: "TS-A07", Analyzer: invariantrefs.Analyzer,
		Severity: SeverityBlocking,
		Title:    "every invariant is asserted in a function outside _test.go files",
		Finish:   invariantrefs.Finish,
	},
	{
		Category: "TS-A09", RuleID: "TS-A09", Analyzer: invariantnegative.Analyzer,
		Severity: SeverityBlocking,
		Title:    "every invariant has a test that violates it",
		Finish:   invariantnegative.Finish,
	},
	{
		Category: "TS-X01", RuleID: "TS-X01", Analyzer: singleimpl.Analyzer,
		Severity: SeverityBlocking,
		Title:    "no interface with exactly one implementation outside _test.go files",
		Finish:   singleimpl.Finish,
	},
}

// facts is the computed-facts channel: one category per pinnable fact.
var facts = []Fact{
	{
		Category: "TS-F01-facts", RuleID: "TS-F01", Analyzer: effects.Analyzer,
		Title: "computed effect sets print in pin syntax under --show-facts",
	},
	{
		Category: "TS-F07-facts", RuleID: "TS-F07", Analyzer: frames.Analyzer,
		Title: "computed frames print in pin syntax under --show-facts",
	},
	{
		Category: "TS-V01-facts", RuleID: "TS-V01", Analyzer: variant.Analyzer,
		Title: "synthesized variants print in pin syntax under --show-facts",
	},
}

// Facts returns every registered fact category.
func Facts() []Fact {
	listed := make([]Fact, len(facts))
	copy(listed, facts)
	return listed
}

// ByFact resolves a diagnostic category to its fact entry, when the
// category is a computed fact rather than a rule.
func ByFact(category string) (Fact, bool) {
	for _, fact := range facts {
		if fact.Category == category {
			return fact, true
		}
	}
	return Fact{}, false
}

// CustomRules returns every registered custom-rule category.
func CustomRules() []CustomRule {
	listed := make([]CustomRule, len(customRules))
	copy(listed, customRules)
	return listed
}

// ByCategory resolves a diagnostic category to its rule entry. Every
// diagnostic an analyzer emits must resolve here or in ByFact (constraint
// 1).
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

// Finishers returns every registered finish function paired with its
// analyzer, one per analyzer, sorted by analyzer name. Only the tiger CLI
// calls them; the plugin and analysistest see Analyzers() alone.
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

// WholeProgram reports whether the named analyzer registers a finish
// function, so its corpus lives in the module layout the finish step can
// run over.
func WholeProgram(analyzerName string) bool {
	for _, rule := range customRules {
		if rule.Analyzer != nil && rule.Analyzer.Name == analyzerName && rule.Finish != nil {
			return true
		}
	}
	return false
}

// RuleIDs returns the distinct analyzer-enforced rule IDs, sorted. The
// corpus meta-test walks these: every rule needs its failure-mode and
// compliant cases.
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

// CountedCodes returns the rule codes registered at advisory severity —
// the codes a budget row may name — mapped to true.
func CountedCodes() map[string]bool {
	codes := map[string]bool{}
	for _, rule := range customRules {
		if rule.Severity == SeverityAdvisory {
			codes[rule.RuleID] = true
		}
	}
	return codes
}

// CorpusDir names the analysistest corpus package for a rule: the rule ID
// lowercased, for example ts-s09.
func CorpusDir(ruleID string) string {
	return strings.ToLower(ruleID)
}
