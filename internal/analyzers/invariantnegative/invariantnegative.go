// Package invariantnegative enforces TS-A09: every declared invariant has
// a test that violates it.
//
// The negative-space proof lives in a _test.go file, possibly in a
// different package than the one declaring the invariant, so — like
// TS-A07 — no single pass sees both the declaration and the proof. The
// per-package pass exports what this package declares and calls; the
// finish step reads every package's contribution back.
package invariantnegative

import (
	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/invariants"
	"github.com/kapetan-io/tiger/internal/finish"
)

type Fact struct {
	Contribution invariants.Contribution
}

func (*Fact) AFact() {}

var Analyzer = &analysis.Analyzer{
	Name:      "invariantnegative",
	Doc:       "TS-A09: every declared invariant has a test that violates it",
	FactTypes: []analysis.Fact{new(Fact)},
	Run:       run,
}

func run(pass *analysis.Pass) (any, error) {
	pass.ExportPackageFact(&Fact{Contribution: invariants.Collect(pass)})
	return nil, nil
}

func Finish(program *finish.Program) ([]analysis.Diagnostic, error) {
	contributions := make([]invariants.Contribution, 0, len(program.PackageFacts))
	for _, fact := range program.PackageFacts {
		contribution, ok := fact.Fact.(*Fact)
		assert.Ok(ok, "a package fact of this analyzer is a Fact")
		contributions = append(contributions, contribution.Contribution)
	}
	diagnostics := []analysis.Diagnostic{}
	demanded := invariants.Defense{Kind: invariants.KindViolates, In: invariants.Tests}
	for _, constant := range invariants.Undefended(program, contributions, demanded) {
		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:      constant.Pos(),
			Category: "TS-A09",
			Message: "TS-A09: no test violates invariant " + invariants.Qualified(constant) +
				" — add a _test.go function that calls assert.Violates(" +
				invariants.Qualified(constant) + ", func() { ... })",
		})
	}
	return diagnostics, nil
}
