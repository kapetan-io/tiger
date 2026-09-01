package drivertest_test

import (
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
	r.reports = append(r.reports, format)
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
