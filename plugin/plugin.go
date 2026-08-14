// Package plugin exposes the registered tiger analyzers to golangci-lint as
// a module plugin, so auto and custom rules run in one pass.
//
// This is a registration shim, deliberately without logic: the analyzer set
// comes from the rule registry — the only path into any driver — and the
// analyzers themselves cannot tell which driver runs them (ADR-0002). If
// this package ever needs a branch, the driver-agnosticism principle has
// been violated somewhere else.
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

// BuildAnalyzers returns every registered wave-1 analyzer.
func (tiger) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return rules.Analyzers(), nil
}

// GetLoadMode requests full type information; wave-1 analyzers read a single
// package's AST and types.
func (tiger) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
