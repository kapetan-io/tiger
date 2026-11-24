// Package finish is the contract between a whole-program rule and the
// driver that runs its finish step (ADR-0010).
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
