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
// Whole-program rules (ADR-0010) — TS-A07 and TS-A09 — decide their
// findings in a finish step only the tiger CLI runs. Under this plugin
// their per-package halves still run and export their facts, so nothing
// breaks, but those two rule codes never appear in a golangci-lint run;
// only `tiger check` reports them.
//
// The plugin also installs the repository's tiger.yaml and tiger.layers.yaml
// (ADR-0002: the driver loads config, analyzers only query it). Both are
// read from the working directory — golangci-lint runs from the module root
// — unless the plugin's settings carry a `config` key naming another
// directory. A load error fails plugin construction with the same one-line
// message tiger check prints. TS-X03's import and missing-entry findings
// surface here for the packages golangci-lint loads; an entry that matches no
// package, and a package with no entry that the run did not load or that
// holds only generated files, do not, because those checks need the module's
// directories and only tiger check runs them (ADR-0029). golangci-lint keys
// its issue cache on the sorted analyzer names, salted with its version
// string, the linters.settings block and go.mod, never on the plugin binary or
// the layer file, so the plugin adds an analyzer that does nothing and whose
// name carries the file's digest: an edited layer file changes the key and the
// cached issues are not replayed. Budgets (tiger.budget.yaml) are not enforced here: the
// plugin API has no end-of-run hook to aggregate counts, so every counted
// finding surfaces as an issue.
//
// Build it with golangci-lint's module plugin mechanism: see .custom-gcl.yml
// at the repository root and run `golangci-lint custom`.
package plugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/config"
	"github.com/kapetan-io/tiger/internal/layerfile"
	"github.com/kapetan-io/tiger/internal/rules"
)

// tiger satisfies register.LinterPlugin with the registry's analyzer set and
// the layer file it installed.
type tiger struct {
	layers *layerfile.File
}

//nolint:gochecknoinits // The golangci-lint plugin contract registers by init.
func init() {
	register.Plugin("tiger", newPlugin)
}

// settings is the plugin's own configuration block under
// linters.settings.custom.tiger.settings.
type settings struct {
	// Config is the directory holding tiger.yaml and tiger.layers.yaml; the
	// working directory when empty.
	Config string `json:"config"`
}

func newPlugin(conf any) (register.LinterPlugin, error) {
	parsed, err := register.DecodeSettings[settings](conf)
	if err != nil {
		return nil, err
	}
	dir := parsed.Config
	if dir == "" {
		dir = "."
	}
	loaded, err := config.Load(dir, rules.Analyzers())
	if err != nil {
		return nil, err
	}
	config.Install(loaded)
	layered, err := layerfile.Load(dir)
	if err != nil {
		return nil, err
	}
	layerfile.Install(layered)
	return tiger{layers: layered}, nil
}

// BuildAnalyzers returns every registered analyzer, with fact diagnostics
// filtered out (the golangci driver's output policy), plus the layer file's
// cache key. Finish functions are never consulted: golangci-lint has no
// end-of-module hook.
func (t tiger) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	analyzers := rules.Analyzers()
	built := make([]*analysis.Analyzer, 0, len(analyzers)+1)
	for _, registered := range analyzers {
		built = append(built, applyOutputPolicy(registered))
	}
	return append(built, layerFileKey(t.layers)), nil
}

// layerFileKey returns an analyzer that reports nothing and whose name
// carries the layer file's digest. The issue cache of golangci-lint is keyed
// on the analyzers' names, so this is how an edit to the layer file reaches
// it.
func layerFileKey(layers *layerfile.File) *analysis.Analyzer {
	digest := layers.Digest()
	const shown = 16
	if len(digest) > shown {
		digest = digest[:shown]
	}
	return &analysis.Analyzer{
		Name: "layerfile_" + digest,
		Doc:  "Names the installed tiger.layers.yaml's digest for golangci-lint's issue cache.",
		Run:  func(*analysis.Pass) (any, error) { return nil, nil },
	}
}

// applyOutputPolicy returns a shallow clone whose Run drops fact diagnostics
// before they reach golangci-lint and ends every rule finding with its
// pattern pointer, the same two decisions tiger check makes. The clone's
// Requires still name the original dependency analyzers, so golangci's own
// resolution and facts plumbing are untouched.
func applyOutputPolicy(registered *analysis.Analyzer) *analysis.Analyzer {
	inner := registered.Run
	clone := *registered
	clone.Run = func(pass *analysis.Pass) (any, error) {
		report := pass.Report
		pass.Report = func(diagnostic analysis.Diagnostic) {
			if _, isFact := rules.ByFact(diagnostic.Category); isFact {
				return
			}
			diagnostic.Message += rules.Pointer(diagnostic.Category)
			report(diagnostic)
		}
		return inner(pass)
	}
	return &clone
}

// GetLoadMode requests full type information; golangci-lint supplies the
// syntax and types buildssa needs to construct SSA for the SSA analyzers.
func (tiger) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
