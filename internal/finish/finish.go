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

// Package is one loaded package as the finish step sees it.
type Package struct {
	Pkg       *types.Package
	TypesInfo *types.Info
	Syntax    []*ast.File
}

// Program is the finish step's input: every loaded package in the stable
// topological order the passes used, one file set positioning all of
// them, and read access to every fact the owning analyzer exported during
// the run — package facts in export order, object facts likewise. Nothing
// else: no results cache, no other analyzer's facts.
type Program struct {
	Fset         *token.FileSet
	Packages     []Package
	PackageFacts []analysis.PackageFact
	ObjectFacts  []analysis.ObjectFact
}

// Lookup returns the loaded package with the given import path. Under the
// test-augmented variant seam ("pkg [pkg.test]") the augmented variant is
// the one loaded, and its path is the plain import path.
func (p *Program) Lookup(path string) (Package, bool) {
	for _, pkg := range p.Packages {
		if pkg.Pkg.Path() == path {
			return pkg, true
		}
	}
	return Package{}, false
}

// Func is one analyzer's finish half. It returns diagnostics in the same
// shape passes emit; every position must resolve inside the loaded module.
// A returned error, like a panic, is an operational failure for the whole
// run — no findings are returned. Determinism is the function's own
// responsibility: the driver hands it packages and facts in a stable
// order, and a function that builds maps must sort before emitting.
type Func func(program *Program) ([]analysis.Diagnostic, error)

// Finisher pairs a finish function with the analyzer whose facts it reads.
type Finisher struct {
	Analyzer *analysis.Analyzer
	Run      Func
}
