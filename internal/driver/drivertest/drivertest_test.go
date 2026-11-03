package drivertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/driver/drivertest"
	"github.com/kapetan-io/tiger/internal/finish"
)

// recorder collects the harness's reports instead of failing a test.
type recorder struct {
	reports []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.reports = append(r.reports, fmt.Sprintf(format, args...))
}

// packageFact is the probe's package fact.
type packageFact struct {
	Path string
}

func (*packageFact) AFact() {}

// probe reports one diagnostic at the package clause of every loaded
// package through the finish step, plus one per-package diagnostic on the
// first import of any file that has one.
func probe() finish.Finisher {
	analyzer := &analysis.Analyzer{
		Name:      "probe",
		Doc:       "exercises the module corpus harness",
		FactTypes: []analysis.Fact{new(packageFact)},
		Run: func(pass *analysis.Pass) (any, error) {
			pass.ExportPackageFact(&packageFact{Path: pass.Pkg.Path()})
			for _, file := range pass.Files {
				if len(file.Imports) > 0 {
					pass.Report(analysis.Diagnostic{
						Pos: file.Imports[0].Pos(), Category: "probe", Message: "probe: import",
					})
				}
			}
			return nil, nil
		},
	}
	return finish.Finisher{
		Analyzer: analyzer,
		Run: func(program *finish.Program) ([]analysis.Diagnostic, error) {
			diagnostics := []analysis.Diagnostic{}
			for _, fact := range program.PackageFacts {
				pkg, _ := program.Lookup(fact.Package.Path())
				diagnostics = append(diagnostics, analysis.Diagnostic{
					Pos:      pkg.Syntax[0].Package,
					Category: "probe",
					Message:  "probe: finished " + fact.Package.Path(),
				})
			}
			return diagnostics, nil
		},
	}
}

// TestRunMatchesWantExpectations covers the harness on a module whose
// expectations all hold.
//
// Goal: every finding — per-package and finish-step alike — matches a
// // want pattern on its line, nothing is reported, and the findings come
// back for further checks.
func TestRunMatchesWantExpectations(t *testing.T) {
	reports := &recorder{}
	findings := drivertest.Run(reports, "testdata/matching", probe())
	assert.Empty(t, reports.reports)
	require.Len(t, findings, 3)
}

// TestRunReportsMismatches covers the harness's two failure directions.
//
// Goal: a finding with no matching expectation and an expectation with no
// matching finding are each reported.
func TestRunReportsMismatches(t *testing.T) {
	reports := &recorder{}
	drivertest.Run(reports, "testdata/mismatched", probe())
	require.Len(t, reports.reports, 2)
	assert.Contains(t, reports.reports[0], "unexpected diagnostic")
	assert.Contains(t, reports.reports[1], "missing diagnostic")
}

// TestRunReportsDriverErrorsWithoutFindings covers a run that cannot
// complete.
//
// Goal: a type error is reported through the Reporter and no findings come
// back, so a caller never mistakes a partial run for a clean one.
func TestRunReportsDriverErrorsWithoutFindings(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module fixture.example/broken\n\ngo 1.26\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "broken.go"),
		[]byte("package broken\n\nvar _ int = \"text\"\n"), 0o600))

	reports := &recorder{}
	findings := drivertest.Run(reports, root, probe())
	require.Len(t, reports.reports, 1)
	assert.Empty(t, findings)
}

// TestRunReportsUnquotedWantPatterns covers a malformed expectation.
//
// Goal: a // want comment whose pattern is not a quoted string is reported
// as a malformed expectation, not silently read as "expects nothing".
func TestRunReportsUnquotedWantPatterns(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module fixture.example/unquoted\n\ngo 1.26\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unquoted.go"),
		[]byte("package unquoted // want probe: finished\n"), 0o600))

	reports := &recorder{}
	drivertest.Run(reports, root, probe())
	require.NotEmpty(t, reports.reports)
	assert.Contains(t, reports.reports[0], "want")
}
