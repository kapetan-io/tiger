package driver_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"

	"github.com/kapetan-io/tiger/internal/driver"
)

// probeFact is the test double's exported fact type. The plumbing this test
// proves does not care what a fact contains, only that it survives the
// cross-package hop intact.
type probeFact struct {
	Path string
}

func (*probeFact) AFact() {}

// newProbe builds a fresh test-double analyzer for one Check call. It
// Requires buildssa (proving dependency-result plumbing), exports a fact on
// every exported top-level function of the package it analyzes (proving
// export), imports that fact back for every direct import's exported
// functions (proving cross-package import), and records the package visit
// order plus one diagnostic per package (proving topological order and
// determinism).
func newProbe(t *testing.T, order *[]string) *analysis.Analyzer {
	t.Helper()
	return &analysis.Analyzer{
		Name: "probe",
		Doc: "test double proving buildssa plumbing, cross-package facts, and topological " +
			"order",
		Requires:  []*analysis.Analyzer{buildssa.Analyzer},
		FactTypes: []analysis.Fact{new(probeFact)},
		Run: func(pass *analysis.Pass) (any, error) {
			ssaResult, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
			require.True(t, ok)
			require.NotNil(t, ssaResult)

			*order = append(*order, pass.Pkg.Path())

			for _, file := range pass.Files {
				for _, decl := range file.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if !ok {
						continue
					}
					if !fn.Name.IsExported() {
						continue
					}
					obj := pass.TypesInfo.Defs[fn.Name]
					pass.ExportObjectFact(obj, &probeFact{Path: pass.Pkg.Path()})
				}
			}

			for _, imp := range pass.Pkg.Imports() {
				for _, name := range imp.Scope().Names() {
					fn, ok := imp.Scope().Lookup(name).(*types.Func)
					if !ok {
						continue
					}
					if !fn.Exported() {
						continue
					}
					got := new(probeFact)
					found := pass.ImportObjectFact(fn, got)
					require.True(t, found)
					require.Equal(t, imp.Path(), got.Path)
				}
			}

			pass.Report(analysis.Diagnostic{
				Pos:      pass.Files[0].Package,
				Category: "probe",
				Message:  "probe: " + pass.Pkg.Path(),
			})
			return nil, nil
		},
	}
}

// wantFindings is the position-sorted output every run must produce.
// "app/app.go" sorts before "dep/dep.go" even though dep is analyzed
// first — topological order runs dependencies before dependents, but the
// final findings are sorted by position, not by visit order.
func wantFindings() []driver.Finding {
	return []driver.Finding{
		{
			Position: token.Position{Filename: "app/app.go", Line: 1, Column: 1},
			Category: "probe",
			Message:  "probe: fixture.example/probe/app",
		},
		{
			Position: token.Position{Filename: "dep/dep.go", Line: 1, Column: 1},
			Category: "probe",
			Message:  "probe: fixture.example/probe/dep",
		},
	}
}

// TestCheckPlumbsDependenciesFactsAndOrder covers the driver evolution
// slice through its public surface, driver.Check.
//
// Goal: buildssa's result reaches every package (ResultOf plumbing), a fact
// exported on an exported function of a dependency package is importable
// while analyzing the package that imports it (cross-package facts), and
// two runs over the same fixture produce identical findings and identical
// package visit order (stable topological order, constraint 5).
func TestCheckPlumbsDependenciesFactsAndOrder(t *testing.T) {
	const root = "testdata/fixtures/probe"
	wantOrder := []string{"fixture.example/probe/dep", "fixture.example/probe/app"}

	var order1 []string
	findings1, err := driver.Check(
		root, []string{"./..."}, []*analysis.Analyzer{newProbe(t, &order1)},
	)
	require.NoError(t, err)
	require.Equal(t, wantOrder, order1)
	require.Equal(t, wantFindings(), findings1)

	var order2 []string
	findings2, err := driver.Check(
		root, []string{"./..."}, []*analysis.Analyzer{newProbe(t, &order2)},
	)
	require.NoError(t, err)
	require.Equal(t, wantOrder, order2)
	require.Equal(t, findings1, findings2)
}
