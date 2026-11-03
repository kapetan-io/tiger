package driver_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
			Package:  "fixture.example/probe/app",
		},
		{
			Position: token.Position{Filename: "dep/dep.go", Line: 1, Column: 1},
			Category: "probe",
			Message:  "probe: fixture.example/probe/dep",
			Package:  "fixture.example/probe/dep",
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
		root, []string{"./..."}, []*analysis.Analyzer{newProbe(t, &order1)}, nil,
	)
	require.NoError(t, err)
	require.Equal(t, wantOrder, order1)
	require.Equal(t, wantFindings(), findings1)

	var order2 []string
	findings2, err := driver.Check(
		root, []string{"./..."}, []*analysis.Analyzer{newProbe(t, &order2)}, nil,
	)
	require.NoError(t, err)
	require.Equal(t, wantOrder, order2)
	require.Equal(t, findings1, findings2)

	report, err := driver.Run(
		root, []string{"./..."}, []*analysis.Analyzer{newProbe(t, &order2)}, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, wantOrder, report.Packages)
	assert.Equal(t, findings1, report.Findings)
}

// TestCheckAnalyzesTestVariantsOnceAndSharesFacts covers the loader's
// test-variant handling through driver.Check.
//
// Goal: a package with in-package and external tests is analyzed once, as
// its test-augmented variant, so no file is reported twice; the external
// test package reports under its own _test path; a real directory named
// like another package's external test ("lib_test") keeps its files; a
// fact exported by the package under test is importable from its external
// test package, which sees the plain import path; and findings inside a
// "Code generated ... DO NOT EDIT." file are dropped.
func TestCheckAnalyzesTestVariantsOnceAndSharesFacts(t *testing.T) {
	root := t.TempDir()
	const module = "fixture.example/variants"
	write := func(name, source string) {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
	}
	write("go.mod", "module "+module+"\n\ngo 1.26\n")
	write("lib/lib.go", "package lib\n\nfunc Exported() {}\n")
	write("lib/inner_test.go", "package lib\n")
	write("lib/outer_test.go",
		"package lib_test\n\nimport \""+module+"/lib\"\n\nvar _ = lib.Exported\n")
	write("lib/gen.go", "// Code generated by hand. DO NOT EDIT.\n\npackage lib\n")
	write("lib_test/real.go", "package real\n")

	analyzer := &analysis.Analyzer{
		Name:      "variants",
		Doc:       "test double reporting once per file and checking facts across the test seam",
		FactTypes: []analysis.Fact{new(probeFact)},
		Run: func(pass *analysis.Pass) (any, error) {
			for _, file := range pass.Files {
				if file.Name.Name == "lib" && pass.Pkg.Path() == module+"/lib" {
					for _, decl := range file.Decls {
						if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.IsExported() {
							obj := pass.TypesInfo.Defs[fn.Name]
							pass.ExportObjectFact(obj, &probeFact{Path: "lib"})
						}
					}
				}
				pass.Report(analysis.Diagnostic{
					Pos: file.Package, Category: "variants", Message: "variants: file",
				})
			}
			if pass.Pkg.Path() == module+"/lib_test" && pass.Pkg.Name() == "lib_test" {
				for _, imp := range pass.Pkg.Imports() {
					fn, ok := imp.Scope().Lookup("Exported").(*types.Func)
					require.True(t, ok)
					require.True(t, pass.ImportObjectFact(fn, new(probeFact)))
				}
			}
			return nil, nil
		},
	}

	findings, err := driver.Check(root, []string{"./..."}, []*analysis.Analyzer{analyzer}, nil)
	require.NoError(t, err)

	got := map[string]string{}
	for _, finding := range findings {
		_, duplicate := got[finding.Position.Filename]
		require.False(t, duplicate)
		got[finding.Position.Filename] = finding.Package
	}
	assert.Equal(t, map[string]string{
		"lib/lib.go":        module + "/lib",
		"lib/inner_test.go": module + "/lib",
		"lib/outer_test.go": module + "/lib_test",
		"lib_test/real.go":  module + "/lib_test",
	}, got)
}

// TestCheckAnalyzerPanicReturnsNoFindings covers behavioral constraint 1:
// never present partial results as a complete run.
//
// Goal: an analyzer pass that panics turns the whole Check into an error
// naming the analyzer and the package, with no findings returned beside it
// from the analyzers that did finish.
func TestCheckAnalyzerPanicReturnsNoFindings(t *testing.T) {
	panicker := &analysis.Analyzer{
		Name: "panicker",
		Doc:  "test double that panics on every package",
		Run: func(*analysis.Pass) (any, error) {
			panic("boom")
		},
	}

	findings, err := driver.Check(
		"testdata/fixtures/probe", []string{"./..."},
		[]*analysis.Analyzer{newProbe(t, new([]string)), panicker}, nil,
	)

	require.ErrorContains(t, err, "analyzer panicker panicked on fixture.example/probe/")
	require.ErrorContains(t, err, "boom")
	require.Nil(t, findings)
}
