// Package plugin exposes the registered tiger analyzers to golangci-lint as
// a module plugin, so auto and custom rules run in one pass.
package plugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/config"
	"github.com/kapetan-io/tiger/internal/layerfile"
	"github.com/kapetan-io/tiger/internal/rules"
)

type tiger struct {
	layers *layerfile.File
}

//nolint:gochecknoinits // The golangci-lint plugin contract registers by init.
func init() {
	register.Plugin("tiger", newPlugin)
}

type settings struct {
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

func (t tiger) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	analyzers := rules.Analyzers()
	built := make([]*analysis.Analyzer, 0, len(analyzers)+1)
	for _, registered := range analyzers {
		built = append(built, applyOutputPolicy(registered))
	}
	return append(built, layerFileKey(t.layers)), nil
}

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

func (tiger) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
