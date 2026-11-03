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
// method is not followed, because there is no body to trace. A constant
// below half the type's maximum is trusted, because it cannot be told from a
// legitimate large count. Both are listed under "Known misses" in
// docs/Tiger Rule Reference.md.
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
	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/mutation"
)

const (
	msgUnbounded = "TS-S02: this value reaches a loop limit with no upper bound where it " +
		"enters — clamp it there, for example min(x, xMax), or reject it with if x > xMax"
	msgHuge = "TS-S02: this value is too large for the loop to ever reach, so it bounds " +
		"nothing — pass a declared maximum instead"
)

const roundsMax = 64

const _ = uint(1<<10 - roundsMax)

type LimitParams struct{ Index []int }

func (*LimitParams) AFact() {}

func (f *LimitParams) String() string { return fmt.Sprint(f.Index) }

type LimitFields struct{ Keys []string }

func (*LimitFields) AFact() {}

func (f *LimitFields) String() string { return fmt.Sprint(f.Keys) }

type BoundedResult struct{ DependsOn []int }

func (*BoundedResult) AFact() {}

func (f *BoundedResult) String() string { return fmt.Sprint(f.DependsOn) }

var Analyzer = &analysis.Analyzer{
	Name: "limitflow",
	Doc:  "TS-S02: every loop limit is a constant, a length, or clamped where it enters.",
	Run:  run,
	FactTypes: []analysis.Fact{
		new(LimitParams), new(LimitFields), new(BoundedResult),
	},
}

type function struct {
	obj           *types.Func
	decl          *ast.FuncDecl
	scope         mutation.Scope
	params        map[*types.Var]int
	literalParams map[*types.Var]bool
}

type state struct {
	pass        *analysis.Pass
	funcs       []*function
	limitParams map[*types.Func]map[int]bool
	limitFields map[string]bool
	sizes       types.Sizes
	ownFields   map[string]bool
	results     map[*types.Func][]int
	sinks       map[*function][]sink
}

func run(pass *analysis.Pass) (any, error) {
	if standardLibrary(pass) {
		return nil, nil
	}
	flow := &state{
		pass: pass, limitParams: map[*types.Func]map[int]bool{},
		limitFields: map[string]bool{}, ownFields: map[string]bool{},
		results: map[*types.Func][]int{}, sinks: map[*function][]sink{},
	}
	for _, fact := range pass.AllPackageFacts() {
		if fields, ok := fact.Fact.(*LimitFields); ok {
			for _, key := range fields.Keys {
				flow.limitFields[key] = true
			}
		}
	}
	flow.collect()
	flow.settle()
	flow.report()
	flow.export()
	return nil, nil
}

func standardLibrary(pass *analysis.Pass) bool {
	if len(pass.Files) == 0 {
		return false
	}
	name := pass.Fset.File(pass.Files[0].Pos()).Name()
	root := filepath.Join(build.Default.GOROOT, "src") + string(filepath.Separator)
	return strings.HasPrefix(name, root)
}

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
		s.sizes = scope.Sizes()
		fn := &function{
			obj: obj, decl: decl, scope: scope, params: map[*types.Var]int{},
			literalParams: scope.Parameters(),
		}
		signature, ok := obj.Type().(*types.Signature)
		if !ok {
			continue
		}
		for i := 0; i < signature.Params().Len(); i++ {
			fn.params[signature.Params().At(i)] = i
			delete(fn.literalParams, signature.Params().At(i))
		}
		s.funcs = append(s.funcs, fn)
		s.sinks[fn] = s.sinksIn(fn)
	}
}

func (s *state) inTestFile(decl *ast.FuncDecl) bool {
	return strings.HasSuffix(s.pass.Fset.File(decl.Pos()).Name(), "_test.go")
}

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

func (s *state) round() bool {
	grew := false
	for _, fn := range s.funcs {
		for _, candidate := range s.sinks[fn] {
			if !s.active(candidate) {
				continue
			}
			result := s.trace(fn, candidate.expr, candidate.mode)
			grew = s.learn(fn, result) || grew
		}
		grew = s.learnResult(fn) || grew
	}
	return grew
}

func (s *state) learn(fn *function, result traceResult) bool {
	grew := false
	for _, index := range result.params {
		if s.limitParams[fn.obj] == nil {
			s.limitParams[fn.obj] = map[int]bool{}
		}
		if !s.limitParams[fn.obj][index] {
			s.limitParams[fn.obj][index] = true
			grew = true
		}
	}
	for _, field := range result.fields {
		key := s.fieldKey(field)
		if key != "" && !s.limitFields[key] {
			s.limitFields[key] = true
			s.ownFields[key] = true
			grew = true
		}
	}
	return grew
}

func (s *state) learnResult(fn *function) bool {
	signature, ok := fn.obj.Type().(*types.Signature)
	if !ok || signature.Results().Len() != 1 ||
		!intrange.IsInteger(signature.Results().At(0).Type()) {
		return false
	}
	var depends []int
	returns, unbounded := 0, false
	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		if _, isLiteral := n.(*ast.FuncLit); isLiteral {
			return false
		}
		stmt, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		returned := s.returnedValue(fn, stmt)
		if returned == nil {
			return true
		}
		returns++
		result := s.trace(fn, returned, downstream)
		if len(result.bad) > 0 || len(result.huge) > 0 {
			unbounded = true
		}
		depends = append(depends, result.params...)
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

func (s *state) returnedValue(fn *function, stmt *ast.ReturnStmt) ast.Expr {
	if len(stmt.Results) == 1 {
		return stmt.Results[0]
	}
	results := fn.decl.Type.Results
	if len(stmt.Results) == 0 && results != nil && len(results.List) == 1 &&
		len(results.List[0].Names) == 1 {
		return results.List[0].Names[0]
	}
	return nil
}

func (s *state) report() {
	reported := map[token.Pos]bool{}
	for _, fn := range s.funcs {
		for _, candidate := range s.sinks[fn] {
			if !s.active(candidate) {
				continue
			}
			result := s.trace(fn, candidate.expr, candidate.mode)
			for _, leaf := range result.bad {
				s.say(reported, leaf, msgUnbounded)
			}
			for _, leaf := range result.huge {
				s.say(reported, leaf, msgHuge)
			}
		}
	}
}

func (s *state) say(reported map[token.Pos]bool, expr ast.Expr, message string) {
	if reported[expr.Pos()] {
		return
	}
	reported[expr.Pos()] = true
	s.pass.Report(analysis.Diagnostic{
		Pos: expr.Pos(), Category: "TS-S02-limit", Message: message,
	})
}

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
