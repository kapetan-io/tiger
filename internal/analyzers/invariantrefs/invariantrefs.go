// Package invariantrefs enforces TS-A07: every declared invariant is
// asserted in at least one function outside _test.go files.
//
// The evidence is spread across packages: an invariant const is declared
// in one package and asserted from the packages that import it, which run
// after it in dependency order and never report back. The per-package
// pass exports what this package declares and calls; the finish step,
// seeing every package's contribution, is the only place both sides meet.
package invariantrefs

import (
	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/invariants"
	"github.com/kapetan-io/tiger/internal/finish"
)

// Fact carries one package's invariant consts and assert-call sites across
// the facts mechanism, for the finish step to read back.
type Fact struct {
	Contribution invariants.Contribution
}

// AFact marks Fact as a go/analysis fact.
func (*Fact) AFact() {}

// Analyzer computes each package's contribution to TS-A07. Its own pass
// never reports: only Finish, run by the tiger CLI after every package has
// been visited, can tell which invariants no production function asserts.
var Analyzer = &analysis.Analyzer{
	Name:      "invariantrefs",
	Doc:       "TS-A07: every declared invariant is asserted outside _test.go files",
	FactTypes: []analysis.Fact{new(Fact)},
	Run:       run,
}

func run(pass *analysis.Pass) (any, error) {
	pass.ExportPackageFact(&Fact{Contribution: invariants.Collect(pass)})
	return nil, nil
}

// Finish reads every package's contribution and reports the invariants no
// function outside a _test.go file asserts (TS-A07).
func Finish(program *finish.Program) ([]analysis.Diagnostic, error) {
	contributions := make([]invariants.Contribution, 0, len(program.PackageFacts))
	for _, fact := range program.PackageFacts {
		contribution, ok := fact.Fact.(*Fact)
		assert.Ok(ok, "a package fact of this analyzer is a Fact")
		contributions = append(contributions, contribution.Contribution)
	}
	diagnostics := []analysis.Diagnostic{}
	demanded := invariants.Defense{Kind: invariants.KindAssert, In: invariants.Production}
	for _, constant := range invariants.Undefended(program, contributions, demanded) {
		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:      constant.Pos(),
			Category: "TS-A07",
			Message: "TS-A07: invariant " + invariants.Qualified(constant) + " is declared " +
				"but no function outside _test.go files asserts it — add " +
				"assert.Invariant(" + invariants.Qualified(constant) + ", ...) where the " +
				"property is established, or delete the declaration",
		})
	}
	return diagnostics, nil
}
