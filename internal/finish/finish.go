// Package finish is the contract between a whole-program rule and the
// driver that runs its finish step (ADR-0010).
//
// A whole-program rule cannot decide its findings inside any single
// go/analysis pass because the evidence is spread across packages that do
// not import each other: an invariant is declared in one package and
// asserted in the packages importing it, which run later and never report
// back. Its analyzer therefore has two halves. The per-package half is an
// ordinary pass that exports a package fact describing what the package
// contributes. The finish half — a Func — runs once after every package has
// been visited, sees every loaded package and every fact its analyzer
// exported, and reports on the whole module.
//
// Only the tiger CLI calls finish functions. golangci-lint's runner and
// analysistest have no end-of-module hook, so under them the per-package
// half still runs and exports its facts, and the rule's findings are
// simply absent.
package finish

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

type Package struct {
	Pkg       *types.Package
	TypesInfo *types.Info
	Syntax    []*ast.File
}

type Program struct {
	Fset         *token.FileSet
	Packages     []Package
	PackageFacts []analysis.PackageFact
	ObjectFacts  []analysis.ObjectFact
}

func (p *Program) Lookup(path string) (Package, bool) {
	for _, pkg := range p.Packages {
		if pkg.Pkg.Path() == path {
			return pkg, true
		}
	}
	return Package{}, false
}

type Func func(program *Program) ([]analysis.Diagnostic, error)

type Finisher struct {
	Analyzer *analysis.Analyzer
	Run      Func
}
