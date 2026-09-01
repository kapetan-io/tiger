// Package plugin exposes the registered tiger analyzers to golangci-lint as
// a module plugin, so auto and custom rules run in one pass.
//
// The analyzer set comes from the rule registry — the only path into any
// driver — and the analyzers themselves cannot tell which driver runs them
// (ADR-0002). The one piece of logic here is exactly the piece ADR-0002
// assigns to a driver: output policy. Computed facts are not rules; they
// print only under tiger check --show-facts. golangci-lint has no such
// channel and treats every diagnostic as an issue (and keeps at most one
// issue per line), so surfacing facts here would both spam adopters and
// crowd real findings off their lines. The plugin therefore drops fact
// diagnostics, the same decision tiger check makes when the flag is absent.
//
// Whole-program rules (ADR-0010) — TS-A07, TS-A09, and TS-X01 — decide
// their findings in a finish step only the tiger CLI runs. Under this
// plugin their per-package halves still run and export their facts, so
// nothing breaks, but those three rule codes never appear in a
// golangci-lint run; only `tiger check` reports them.
//
// Build it with golangci-lint's module plugin mechanism: see .custom-gcl.yml
// at the repository root and run `golangci-lint custom`.
package plugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/rules"
)

// tiger satisfies register.LinterPlugin with the registry's analyzer set.
type tiger struct{}

//nolint:gochecknoinits // The golangci-lint plugin contract registers by init.
func init() {
	register.Plugin("tiger", newPlugin)
}

func newPlugin(conf any) (register.LinterPlugin, error) {
	return tiger{}, nil
}

// BuildAnalyzers returns every registered analyzer, with fact diagnostics
// filtered out (the golangci driver's output policy). Finish functions are
// never consulted: golangci-lint has no end-of-module hook.
func (tiger) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	analyzers := rules.Analyzers()
	built := make([]*analysis.Analyzer, 0, len(analyzers))
	for _, registered := range analyzers {
		built = append(built, quietFacts(registered))
	}
	return built, nil
}

// quietFacts returns the analyzer as-is when it emits no fact category,
// else a shallow clone whose Run drops fact diagnostics before they reach
// golangci-lint. The clone's Requires still
// name the original dependency analyzers, so golangci's own resolution and
// facts plumbing are untouched.
func quietFacts(registered *analysis.Analyzer) *analysis.Analyzer {
	if !emitsFacts(registered) {
		return registered
	}
	inner := registered.Run
	clone := *registered
	clone.Run = func(pass *analysis.Pass) (any, error) {
		report := pass.Report
		pass.Report = func(diagnostic analysis.Diagnostic) {
			if _, isFact := rules.ByFact(diagnostic.Category); isFact {
				return
			}
			report(diagnostic)
		}
		return inner(pass)
	}
	return &clone
}

// emitsFacts reports whether the analyzer owns a fact category.
func emitsFacts(registered *analysis.Analyzer) bool {
	for _, fact := range rules.Facts() {
		if fact.Analyzer == registered {
			return true
		}
	}
	return false
}

// GetLoadMode requests full type information; golangci-lint supplies the
// syntax and types buildssa needs to construct SSA for the SSA analyzers.
func (tiger) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
