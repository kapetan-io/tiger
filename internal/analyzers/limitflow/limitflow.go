// Package limitflow enforces the limit half of TS-S02: every loop limit has
// an upper bound where it enters the program.
//
// A loop that counts up to a number is only as bounded as that number. The
// analyzer traces each loop limit back to where it enters: through
// parameters, struct fields, local variables, and function results, across
// packages. Every value written into a limit must be a constant, a length,
// another bounded limit, or a value clamped against a declared maximum, such
// as min(config.Spins, spinsMax) or an if count > objectsMax that rejects. A
// function that returns a clamped value carries the bound to its callers. A
// call that passes math.MaxInt, the type's maximum, or a constant at least
// half of it is a finding too.
//
// Three facts cross packages. LimitParams marks the parameters of a function
// that end up bounding a loop, so a caller's argument is judged where it is
// passed. LimitFields marks the struct fields that bound a loop, so every
// write into the field is judged where it is written. BoundedResult marks a
// function whose result is bounded whenever the listed parameters are.
//
// A limit that reaches a loop through a function value or an interface
// method is not followed, and a constant below half the type's maximum is
// trusted. Both are recorded misses (ADR-0013).
package limitflow

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
)

const (
	msgUnbounded = "TS-S02: this value bounds a loop but has no upper bound where it enters — " +
		"clamp it there, for example min(x, xMax), or reject it with if x > xMax"
	msgHuge = "TS-S02: this value is too large for the loop to ever reach, so it bounds " +
		"nothing — pass a declared maximum instead"
)

// roundsMax bounds how many times the trace repeats while facts spread
// through callers.
const roundsMax = 64

const _ = uint(1<<10 - roundsMax)

// LimitParams marks the parameters of a function that bound a loop.
type LimitParams struct{ Index []int }

// AFact marks LimitParams as an analysis fact.
func (*LimitParams) AFact() {}

func (f *LimitParams) String() string { return fmt.Sprint(f.Index) }

// LimitFields lists the struct fields (package path, type, field) that bound
// a loop. It is a package fact because the field may belong to another
// package than the loop.
type LimitFields struct{ Keys []string }

// AFact marks LimitFields as an analysis fact.
func (*LimitFields) AFact() {}

func (f *LimitFields) String() string { return fmt.Sprint(f.Keys) }

// BoundedResult marks a function whose result is bounded whenever the
// parameters it lists are.
type BoundedResult struct{ DependsOn []int }

// AFact marks BoundedResult as an analysis fact.
func (*BoundedResult) AFact() {}

func (f *BoundedResult) String() string { return fmt.Sprint(f.DependsOn) }

// Analyzer enforces the limit half of TS-S02: every loop limit has an upper
// bound where it enters the program.
var Analyzer = &analysis.Analyzer{
	Name: "limitflow",
	Doc:  "TS-S02: every loop limit is a constant, a length, or clamped where it enters.",
	Run:  run,
	FactTypes: []analysis.Fact{
		new(LimitParams), new(LimitFields), new(BoundedResult),
	},
}

// function is one declaration with a body, the unit the trace works in.
type function struct {
	obj    *types.Func
	decl   *ast.FuncDecl
	scope  mutation.Scope
	params map[*types.Var]int
}

// state is everything the trace knows about the package under analysis and,
// through facts, the packages it imports.
type state struct {
	pass        *analysis.Pass
	funcs       []*function
	limitParams map[*types.Func]map[int]bool
	limitFields map[string]bool
	ownFields   map[string]bool
	results     map[*types.Func][]int
	sinks       map[*function][]sink
}

func run(pass *analysis.Pass) (any, error) {
	if standardLibrary(pass) {
		return nil, nil
	}
	found := &state{
		pass: pass, limitParams: map[*types.Func]map[int]bool{},
		limitFields: map[string]bool{}, ownFields: map[string]bool{},
		results: map[*types.Func][]int{}, sinks: map[*function][]sink{},
	}
	for _, fact := range pass.AllPackageFacts() {
		if fields, ok := fact.Fact.(*LimitFields); ok {
			for _, key := range fields.Keys {
				found.limitFields[key] = true
			}
		}
	}
	found.collect()
	found.settle()
	found.report()
	found.export()
	return nil, nil
}

// standardLibrary reports whether the package under analysis ships with Go.
// Its loops are not ours to judge, and nothing in a caller's module can write
// into its limits, so it exports no facts.
func standardLibrary(pass *analysis.Pass) bool {
	if len(pass.Files) == 0 {
		return false
	}
	name := pass.Fset.File(pass.Files[0].Pos()).Name()
	return strings.HasPrefix(name, filepath.Join(build.Default.GOROOT, "src")+string(filepath.Separator))
}

// collect gathers the declarations with bodies and the places in each where
// a value can become a loop limit.
func (s *state) collect() {
	for _, scope := range mutation.Declarations(s.pass) {
		decl, ok := scope.Root().(*ast.FuncDecl)
		if !ok || decl.Body == nil || s.inTestFile(decl) {
			continue
		}
		obj, ok := s.pass.TypesInfo.Defs[decl.Name].(*types.Func)
		if !ok {
			continue
		}
		fn := &function{obj: obj, decl: decl, scope: scope, params: map[*types.Var]int{}}
		signature, _ := obj.Type().(*types.Signature)
		for i := 0; i < signature.Params().Len(); i++ {
			fn.params[signature.Params().At(i)] = i
		}
		s.funcs = append(s.funcs, fn)
		s.sinks[fn] = s.sinksIn(fn)
	}
}

// inTestFile reports whether decl sits in a _test.go file. A test's table
// values are not input from outside, so no limit in one is traced.
func (s *state) inTestFile(decl *ast.FuncDecl) bool {
	return strings.HasSuffix(s.pass.Fset.File(decl.Pos()).Name(), "_test.go")
}

// settle repeats the trace until no new limit parameter, limit field, or
// bounded result turns up. Facts only grow, so it ends; roundsMax is the
// guard if that ever stops being true.
func (s *state) settle() {
	settled := false
	for round := 0; round < roundsMax; round++ {
		if !s.round() {
			settled = true
			break
		}
	}
	assert.Ok(settled, "limitflow: trace did not settle within roundsMax rounds")
}

// round traces every sink and every result once and reports whether any
// fact grew.
func (s *state) round() bool {
	grew := false
	for _, fn := range s.funcs {
		for _, found := range s.sinks[fn] {
			if !s.active(found) {
				continue
			}
			outcome := s.trace(fn, found.expr, found.mode)
			grew = s.learn(fn, outcome) || grew
		}
		grew = s.learnResult(fn) || grew
	}
	return grew
}

// learn records what a traced sink teaches: the parameters of fn that feed
// it, and the limit fields it reads.
func (s *state) learn(fn *function, outcome outcome) bool {
	grew := false
	for _, index := range outcome.params {
		if s.limitParams[fn.obj] == nil {
			s.limitParams[fn.obj] = map[int]bool{}
		}
		if !s.limitParams[fn.obj][index] {
			s.limitParams[fn.obj][index] = true
			grew = true
		}
	}
	for _, field := range outcome.fields {
		key := s.fieldKey(field)
		if !s.limitFields[key] {
			s.limitFields[key] = true
			s.ownFields[key] = true
			grew = true
		}
	}
	return grew
}

// learnResult records whether fn returns a value bounded whenever some of
// its parameters are.
func (s *state) learnResult(fn *function) bool {
	signature, _ := fn.obj.Type().(*types.Signature)
	if signature.Results().Len() != 1 || !isInteger(signature.Results().At(0).Type()) {
		return false
	}
	var depends []int
	returns, unbounded := 0, false
	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		if _, isLiteral := n.(*ast.FuncLit); isLiteral {
			return false
		}
		stmt, ok := n.(*ast.ReturnStmt)
		if !ok || len(stmt.Results) != 1 {
			return true
		}
		returns++
		outcome := s.trace(fn, stmt.Results[0], indirect)
		if len(outcome.bad) > 0 || len(outcome.huge) > 0 {
			unbounded = true
		}
		depends = append(depends, outcome.params...)
		return !unbounded
	})
	if returns == 0 || unbounded {
		return false
	}
	sort.Ints(depends)
	depends = slices.Compact(depends)
	if previous, known := s.results[fn.obj]; known && slices.Equal(previous, depends) {
		return false
	}
	s.results[fn.obj] = depends
	return true
}

// report traces every active sink once more, with the facts settled, and
// reports each value that enters with no bound or too large to reach.
func (s *state) report() {
	reported := map[token.Pos]bool{}
	for _, fn := range s.funcs {
		for _, found := range s.sinks[fn] {
			if !s.active(found) {
				continue
			}
			outcome := s.trace(fn, found.expr, found.mode)
			for _, leaf := range outcome.bad {
				s.say(reported, leaf, msgUnbounded)
			}
			for _, leaf := range outcome.huge {
				s.say(reported, leaf, msgHuge)
			}
		}
	}
}

// say reports message at expr once.
func (s *state) say(reported map[token.Pos]bool, expr ast.Expr, message string) {
	if reported[expr.Pos()] {
		return
	}
	reported[expr.Pos()] = true
	s.pass.Report(analysis.Diagnostic{
		Pos: expr.Pos(), Category: "TS-S02-limit", Message: message,
	})
}

// export publishes the facts other packages need.
func (s *state) export() {
	for _, fn := range s.funcs {
		if indexes, ok := s.limitParams[fn.obj]; ok {
			s.pass.ExportObjectFact(fn.obj, &LimitParams{Index: slices.Sorted(maps.Keys(indexes))})
		}
		if depends, ok := s.results[fn.obj]; ok {
			s.pass.ExportObjectFact(fn.obj, &BoundedResult{DependsOn: depends})
		}
	}
	if len(s.ownFields) > 0 {
		s.pass.ExportPackageFact(&LimitFields{Keys: slices.Sorted(maps.Keys(s.ownFields))})
	}
}

// limitParamsOf returns the parameters of fn that bound a loop, from this
// package's trace or from an imported package's fact.
func (s *state) limitParamsOf(fn *types.Func) []int {
	if known, ok := s.limitParams[fn]; ok {
		return slices.Sorted(maps.Keys(known))
	}
	var imported LimitParams
	if s.pass.ImportObjectFact(fn, &imported) {
		return imported.Index
	}
	return nil
}

// resultOf returns the parameters fn's result depends on, and whether fn
// returns a bounded result at all.
func (s *state) resultOf(fn *types.Func) ([]int, bool) {
	if depends, ok := s.results[fn]; ok {
		return depends, true
	}
	var imported BoundedResult
	if s.pass.ImportObjectFact(fn, &imported) {
		return imported.DependsOn, true
	}
	return nil, false
}

// isInteger reports whether t is an integer type.
func isInteger(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}

// sizes returns the target's type sizes, falling back to the common 64-bit
// layout when the driver supplied none.
func (s *state) sizes() types.Sizes {
	if s.pass.TypesSizes != nil {
		return s.pass.TypesSizes
	}
	return types.SizesFor("gc", "amd64")
}
