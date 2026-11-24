// Package variant enforces TS-V01: every loop that is not structurally
// terminating (RangeStmt) or counted (a sound counter, see
// internal/analyzers/internal/counter) must carry a variant — a linear
// integer ranking expression proven to strictly decrease on every back edge
// and bounded below — either synthesized by this analyzer or pinned by the
// author.
package variant

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/cancel"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
	"github.com/kapetan-io/tiger/internal/directive"
	"github.com/kapetan-io/tiger/internal/facts"
	"github.com/kapetan-io/tiger/internal/placement"
)

const (
	msgNoVariant = "TS-V01: tiger can't prove this loop ends: nothing in its condition shrinks " +
		"on every pass — add a cap (for tries := 0; tries < max; tries++ { if done { break } " +
		"... }) that fails when the cap is hit, or say what shrinks with //tiger:variant <expr>"
	msgUnverifiedPinFmt = "TS-V01: //tiger:variant %s doesn't provably shrink on every pass " +
		"(%s) — add a cap (for tries := 0; tries < max; tries++) that fails when " +
		"hit, or name an expression that does shrink"
	msgPointlessPin = "TS-V01: this loop has a //tiger:variant comment but ends on its own (a " +
		"range or counted loop), so the comment says nothing — remove it"
)

type Verified map[*ast.ForStmt]bool

var Analyzer = &analysis.Analyzer{
	Name:       "variant",
	Doc:        "TS-V01: every loop has a synthesized or verified pinned variant.",
	Run:        run,
	ResultType: reflect.TypeOf(Verified{}),
}

type checker struct {
	pass     *analysis.Pass
	scope    mutation.Scope
	set      placement.Set
	verified Verified
	function string
	params   map[*types.Var]bool
}

func run(pass *analysis.Pass) (any, error) {
	set := placement.Collect(pass.Fset, pass.Files)
	verified := Verified{}
	for _, scope := range mutation.Declarations(pass) {
		inspectDecl(&checker{
			pass: pass, scope: scope, set: set, verified: verified,
			params: scope.Parameters(),
		})
	}
	return verified, nil
}

func inspectDecl(check *checker) {
	switch d := check.scope.Root().(type) {
	case *ast.FuncDecl:
		if d.Body != nil {
			check.function = d.Name.Name
			check.inspectLoops(d.Body)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) == 0 {
				continue
			}
			check.function = value.Names[0].Name
			for _, expr := range value.Values {
				check.inspectLoops(expr)
			}
		}
	}
}

func (c *checker) inspectLoops(root ast.Node) {
	ast.Inspect(root, func(node ast.Node) bool {
		switch loop := node.(type) {
		case *ast.RangeStmt:
			c.checkOutOfScopePin(loop)
		case *ast.ForStmt:
			c.checkFor(loop)
		}
		return true
	})
}

func (c *checker) checkFor(loop *ast.ForStmt) {
	if loop.Cond == nil {
		c.checkOutOfScopePin(loop)
		return
	}
	if len(counter.Terms(c.scope, loop)) > 0 {
		c.checkOutOfScopePin(loop)
		return
	}
	if cancel.UntilCancelled(c.pass.TypesInfo, c.params, loop.Cond) {
		c.checkOutOfScopePin(loop)
		return
	}
	pinned := c.set.At(loop, "variant")
	if len(pinned) > 0 {
		c.checkPinned(loop, pinned[0])
		return
	}
	c.checkUnpinned(loop)
}

func (c *checker) checkOutOfScopePin(loop ast.Node) {
	if len(c.set.At(loop, "variant")) == 0 {
		return
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-V01",
		Message:  msgPointlessPin,
	})
}

func (c *checker) checkPinned(loop *ast.ForStmt, pin placement.Pin) {
	parsed, err := directive.ParseVariant(pin.Directive.Args)
	if err != nil {
		assert.Unreachable("placement.Collect only yields well-formed variant pins")
		return
	}
	result := c.verifyPinned(loop, parsed)
	if result.ok {
		c.verified[loop] = true
		return
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-V01",
		Message:  fmt.Sprintf(msgUnverifiedPinFmt, directive.FormatVariant(parsed), result.reason),
	})
}

func (c *checker) checkUnpinned(loop *ast.ForStmt) {
	candidate, ok := c.synthesize(loop)
	if ok {
		c.verified[loop] = true
		c.pass.Report(analysis.Diagnostic{
			Pos:      loop.Pos(),
			Category: "TS-V01-facts",
			Message: facts.Message(facts.Fact{
				RuleID: "TS-V01", Function: c.function,
				Directive: directive.Directive{
					Verb: "variant", Args: directive.FormatVariant(candidate),
				},
			}),
		})
		return
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-V01",
		Message:  msgNoVariant,
	})
}
