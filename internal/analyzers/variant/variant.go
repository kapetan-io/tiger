// Package variant enforces TS-V01: every loop that is not structurally
// terminating (RangeStmt) or counted (a sound counter, see
// internal/analyzers/internal/counter) must carry a variant — a linear
// integer ranking expression proven to strictly decrease on every back edge
// and bounded below — either synthesized by this analyzer or pinned by the
// author.
//
// The fact lifecycle is synthesized -> reported -> pinnable. For every
// in-scope loop the analyzer first tries to synthesize a variant from a
// closed set of Cond shapes (len(s) > c, i < n, i > c, low < high, a struct
// field in place of any variable, and any of those as a top-level && term)
// paired with an unconditional move at the top level of the loop body or in
// the Post clause: a fixed step (i++, i += k, s = s[k:], s = s[:len(s)-k]),
// a division or shift by a constant under a strict > 0 condition, or a
// slice by the length of a part shown not empty. Nothing may invalidate the
// move: any other write to the measured variable or field, a pointer to it
// or a closure that writes it made anywhere in the declaration, a call that
// receives the struct holding a measured field, or a continue, labeled
// continue or goto that could skip the move. Variables are matched by
// object identity, so a shadowing copy never counts. A counter that moves
// toward a limit at the edge of its type's range wraps around instead of
// arriving, so that defeats the proof too.
//
// Synthesis success is a reported fact (TS-V01-facts, printed only under
// --show-facts): the loop is proven with no annotation, and the printed line
// is exact pin syntax (directive.FormatVariant), so ENG-151 can freeze it
// into a //tiger:variant pin by pasting it.
//
// A pin replaces synthesis with the author's own ranking, verified by the
// identical discipline: each term of the pinned expression classifies as
// strictly decreasing, strictly increasing, constant, or unknown against
// the same body rules, and the pin verifies only when every positive term
// is non-increasing, every negative term is non-decreasing, at least one
// term moves strictly, and none is unknown. A pin the analyzer cannot
// verify is blocking even when the loop plausibly terminates: a pin
// freezes the analyzer's own computed verification, in both directions —
// the same bidirectional-exactness principle TS-F01 applies to effects —
// so an unverifiable pin is not a weaker claim than an unpinned loop, it is
// a broken contract. A pin on a loop that needs no variant at all (a
// RangeStmt, a counted ForStmt) is blocking for the same reason: it
// states a fact the analyzer never checks, so it means nothing and must be
// removed.
//
// A loop with neither a synthesized nor a verified pinned variant is
// blocking, and the finding always names the same rewrite: an explicit
// iteration cap whose own counter is itself a synthesizable variant, with an
// assert on exhaustion. That rewrite is always available, so no
// //tiger:variant waiver exists. The finding leads with the cap, and offers a
// pin only as a way to state the loop's own ranking, which the analyzer then
// verifies.
//
// The loops with a verified variant, synthesized or pinned, are this
// analyzer's result: boundedloop (TS-S02) accepts them as bounded.
//
// Scope is AST and go/types only, mirroring boundedloop's structural
// model: a RangeStmt is structurally terminating and out of scope (a stray
// pin on one is itself the finding); a ForStmt with Cond == nil is
// boundedloop's TS-S02/TS-S03 territory, as is a loop that runs until a
// context parameter is cancelled; a counted ForStmt (a sound counter
// in its condition) is bounded by boundedloop, not ranked here.
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
	"github.com/kapetan-io/tiger/internal/pins"
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

// Verified is the set of loops with a verified variant, synthesized or
// pinned.
type Verified map[*ast.ForStmt]bool

// Analyzer enforces TS-V01: every unbounded loop has a synthesized or
// verified pinned variant.
var Analyzer = &analysis.Analyzer{
	Name:       "variant",
	Doc:        "TS-V01: every loop has a synthesized or verified pinned variant.",
	Run:        run,
	ResultType: reflect.TypeOf(Verified{}),
}

// checker holds what judging the loops of one declaration needs.
type checker struct {
	pass     *analysis.Pass
	scope    mutation.Scope
	set      pins.Set
	verified Verified
	function string
	params   map[*types.Var]bool
}

func run(pass *analysis.Pass) (any, error) {
	set := pins.Collect(pass.Fset, pass.Files)
	verified := Verified{}
	for _, scope := range mutation.Declarations(pass) {
		inspectDecl(&checker{
			pass: pass, scope: scope, set: set, verified: verified,
			params: scope.Parameters(),
		})
	}
	return verified, nil
}

// inspectDecl walks one top-level declaration's loops, carrying the name
// the fact message attributes them to: the function declaration's own
// name, or — for a loop inside a function literal in a value spec — the
// named entity whose initializer holds it.
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

// inspectLoops applies the loop checks to every loop beneath root.
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

// checkFor classifies a for statement: out of scope (no Cond, counted, or
// running until a context parameter is cancelled — a stray pin on any of
// these is itself a finding), pinned (verify the pin), or unpinned (attempt
// synthesis).
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

// checkOutOfScopePin reports a variant pin bound to a loop that needs no
// variant: a RangeStmt, a ForStmt with no Cond, a counted ForStmt, or one that
// runs until a context parameter is cancelled.
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

// checkPinned verifies a pinned variant against the loop it binds to,
// reporting a blocking finding only when verification fails.
func (c *checker) checkPinned(loop *ast.ForStmt, pin pins.Pin) {
	parsed, err := directive.ParseVariant(pin.Directive.Args)
	if err != nil {
		assert.Unreachable("pins.Collect only yields well-formed variant pins")
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

// checkUnpinned attempts synthesis: success is a reported fact in pin
// syntax, failure is a blocking finding naming the counter-cap rewrite.
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
