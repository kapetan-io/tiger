package driver_test

import (
	"errors"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/finish"
)

// pkgFact is the finish probe's package fact: the path of the package that
// exported it.
type pkgFact struct {
	Path string
}

func (*pkgFact) AFact() {}

// newFinishProbe builds a test-double analyzer that exports one package
// fact per package, paired with a finish function that records what the
// driver handed it and emits one diagnostic per package fact at that
// package's first file.
func newFinishProbe(seen *finish.Program) finish.Finisher {
	analyzer := &analysis.Analyzer{
		Name:      "finishprobe",
		Doc:       "test double proving the finish step's inputs and outputs",
		FactTypes: []analysis.Fact{new(pkgFact)},
		Run: func(pass *analysis.Pass) (any, error) {
			pass.ExportPackageFact(&pkgFact{Path: pass.Pkg.Path()})
			return nil, nil
		},
	}
	return finish.Finisher{
		Analyzer: analyzer,
		Run: func(program *finish.Program) ([]analysis.Diagnostic, error) {
			*seen = *program
			diagnostics := []analysis.Diagnostic{}
			for _, fact := range program.PackageFacts {
				pkg, found := program.Lookup(fact.Package.Path())
				if !found {
					return nil, errors.New("fact for an unloaded package")
				}
				exported, ok := fact.Fact.(*pkgFact)
				if !ok {
					return nil, errors.New("fact of the wrong type")
				}
				diagnostics = append(diagnostics, analysis.Diagnostic{
					Pos:      pkg.Syntax[0].Package,
					Category: "finishprobe",
					Message:  "finishprobe: " + exported.Path,
				})
			}
			return diagnostics, nil
		},
	}
}

// TestCheckRunsFinishersAfterEveryPackage covers the finish step through
// driver.Check.
//
// Goal: the finish function receives every loaded package in the same
// topological order the passes used and every package fact its analyzer
// exported in export order, its diagnostics merge into the sorted
// findings, and two runs produce identical output.
func TestCheckRunsFinishersAfterEveryPackage(t *testing.T) {
	const root = "testdata/fixtures/probe"
	var seen finish.Program
	probe := newFinishProbe(&seen)
	findings, err := driver.Check(
		root, []string{"./..."}, []*analysis.Analyzer{probe.Analyzer}, []finish.Finisher{probe},
	)
	require.NoError(t, err)

	paths := []string{}
	for _, pkg := range seen.Packages {
		paths = append(paths, pkg.Pkg.Path())
		require.NotNil(t, pkg.TypesInfo)
		require.NotEmpty(t, pkg.Syntax)
	}
	require.Equal(t, []string{"fixture.example/probe/dep", "fixture.example/probe/app"}, paths)
	factPaths := []string{}
	for _, fact := range seen.PackageFacts {
		exported, ok := fact.Fact.(*pkgFact)
		require.True(t, ok)
		factPaths = append(factPaths, exported.Path)
	}
	require.Equal(t, []string{"fixture.example/probe/dep", "fixture.example/probe/app"}, factPaths)
	require.Equal(t, []driver.Finding{
		{
			Position: token.Position{Filename: "app/app.go", Line: 1, Column: 1},
			Category: "finishprobe",
			Message:  "finishprobe: fixture.example/probe/app",
		},
		{
			Position: token.Position{Filename: "dep/dep.go", Line: 1, Column: 1},
			Category: "finishprobe",
			Message:  "finishprobe: fixture.example/probe/dep",
		},
	}, findings)

	var again finish.Program
	probe2 := newFinishProbe(&again)
	findings2, err := driver.Check(
		root, []string{"./..."}, []*analysis.Analyzer{probe2.Analyzer}, []finish.Finisher{probe2},
	)
	require.NoError(t, err)
	require.Equal(t, findings, findings2)
}

// TestCheckFinisherOnlySeesItsOwnFacts covers the finish contract's
// "no other analyzer's facts" clause.
//
// Goal: a finish function registered for an analyzer that exported no
// facts receives none, even though another analyzer exported facts in the
// same run.
func TestCheckFinisherOnlySeesItsOwnFacts(t *testing.T) {
	var seen finish.Program
	exporter := newFinishProbe(&seen)
	var starved finish.Program
	quiet := finish.Finisher{
		Analyzer: &analysis.Analyzer{
			Name: "quiet",
			Doc:  "exports nothing",
			Run:  func(pass *analysis.Pass) (any, error) { return nil, nil },
		},
		Run: func(program *finish.Program) ([]analysis.Diagnostic, error) {
			starved = *program
			return nil, nil
		},
	}
	_, err := driver.Check(
		"testdata/fixtures/probe", []string{"./..."},
		[]*analysis.Analyzer{exporter.Analyzer, quiet.Analyzer},
		[]finish.Finisher{exporter, quiet},
	)
	require.NoError(t, err)
	require.Len(t, seen.PackageFacts, 2)
	require.Empty(t, starved.PackageFacts)
	require.Len(t, starved.Packages, 2)
}

// TestCheckFinisherFailureReturnsNoFindings covers behavioral constraint
// 1: never present partial results as a complete run.
//
// Goal: a finish function that panics or returns an error turns the whole
// Check into an error naming the analyzer, with no findings — not even the
// per-package passes' — returned alongside.
func TestCheckFinisherFailureReturnsNoFindings(t *testing.T) {
	for _, test := range []struct {
		name    string
		run     finish.Func
		wantErr string
	}{
		{
			name: "Panic",
			run: func(*finish.Program) ([]analysis.Diagnostic, error) {
				assert.Fail("boom")
				return nil, nil
			},
			wantErr: "finish step finishprobe panicked: assertion failed: boom",
		},
		{
			name: "Error",
			run: func(*finish.Program) ([]analysis.Diagnostic, error) {
				return nil, errors.New("split brain")
			},
			wantErr: "finish step finishprobe failed: split brain",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var seen finish.Program
			probe := newFinishProbe(&seen)
			probe.Run = test.run
			findings, err := driver.Check(
				"testdata/fixtures/probe", []string{"./..."},
				[]*analysis.Analyzer{probe.Analyzer, newProbe(t, new([]string))},
				[]finish.Finisher{probe},
			)
			require.ErrorContains(t, err, test.wantErr)
			require.Nil(t, findings)
		})
	}
}

// TestCheckRejectsFinishDiagnosticOutsideTheModule covers state invariant
// 1: every finding is positioned in the module under check.
//
// Goal: a finish diagnostic with no position, or positioned in a file
// outside the loaded packages, is an operational error, never a finding.
func TestCheckRejectsFinishDiagnosticOutsideTheModule(t *testing.T) {
	for _, test := range []struct {
		name string
		pos  func(program *finish.Program) token.Pos
	}{
		{
			name: "NoPosition",
			pos:  func(*finish.Program) token.Pos { return token.NoPos },
		},
		{
			name: "Dependency",
			pos: func(program *finish.Program) token.Pos {
				// The fmt package is type-checked but never loaded as a
				// package under check; its objects position outside the
				// module.
				for _, pkg := range program.Packages {
					for _, imported := range pkg.Pkg.Imports() {
						if imported.Path() == "fmt" {
							return imported.Scope().Lookup("Println").Pos()
						}
					}
				}
				return token.NoPos
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var seen finish.Program
			probe := newFinishProbe(&seen)
			probe.Run = func(program *finish.Program) ([]analysis.Diagnostic, error) {
				return []analysis.Diagnostic{{
					Pos: test.pos(program), Category: "finishprobe", Message: "finishprobe: x",
				}}, nil
			}
			findings, err := driver.Check(
				"testdata/fixtures/outside", []string{"./..."},
				[]*analysis.Analyzer{probe.Analyzer}, []finish.Finisher{probe},
			)
			require.ErrorContains(t, err, "finish step finishprobe reported a finding outside")
			require.Nil(t, findings)
		})
	}
}
