// Package compoundcond enforces TS-S06, TS-S07, and TS-S08: simple
// conditions, split assertions, asserted default arms.
package compoundcond

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-S06, TS-S07, and TS-S08: simple conditions, split
// assertions, asserted default arms.
var Analyzer = &analysis.Analyzer{
	Name: "compoundcond",
	Doc:  "TS-S06, TS-S07, and TS-S08: simple conditions, split assertions, asserted default arms.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if ifStmt, ok := node.(*ast.IfStmt); ok {
				reportMixedLogic(pass, ifStmt.Cond)
			}
			if forStmt, ok := node.(*ast.ForStmt); ok {
				reportMixedLogic(pass, forStmt.Cond)
			}
			if call, ok := node.(*ast.CallExpr); ok {
				reportSplitAssertion(pass, call)
			}
			if switchStmt, ok := node.(*ast.SwitchStmt); ok {
				reportClosedSwitch(pass, switchStmt)
			}
			return true
		})
	}
	return nil, nil
}

// logicShape records which logical connectives appear in a boolean
// expression tree, per TS-S06's definition of that tree: BinaryExpr nodes
// joined by && or ||, seen through parens and unary !. Anything else (a
// call, a comparison, an identifier) is a leaf.
type logicShape struct {
	hasAnd bool
	hasOr  bool
}

func scanLogic(expr ast.Expr) logicShape {
	if paren, ok := expr.(*ast.ParenExpr); ok {
		return scanLogic(paren.X)
	}
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
		return scanLogic(unary.X)
	}
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return logicShape{}
	}
	if binary.Op == token.LAND {
		left, right := scanLogic(binary.X), scanLogic(binary.Y)
		return logicShape{hasAnd: true, hasOr: left.hasOr || right.hasOr}
	}
	if binary.Op == token.LOR {
		left, right := scanLogic(binary.X), scanLogic(binary.Y)
		return logicShape{hasAnd: left.hasAnd || right.hasAnd, hasOr: true}
	}
	return logicShape{}
}

// reportMixedLogic fires TS-S06 when cond's boolean expression tree mixes
// && and ||. A nil cond (a ForStmt with no condition) is silent.
func reportMixedLogic(pass *analysis.Pass, cond ast.Expr) {
	if cond == nil {
		return
	}
	shape := scanLogic(cond)
	if !shape.hasAnd || !shape.hasOr {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      cond.Pos(),
		Category: "TS-S06",
		Message: "TS-S06: condition mixes && and || — split into nested if/else (one logical " +
			"operator per condition) so each case is explicit and the else has somewhere to live",
	})
}

// calleeName resolves call's function to a *types.Func and reports its
// unqualified name, but only when that function belongs to a package named
// "assert" — the sanctioned home for Ok, Invariant, and Unreachable.
func calleeName(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	var ident *ast.Ident
	if plain, ok := call.Fun.(*ast.Ident); ok {
		ident = plain
	}
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		ident = selector.Sel
	}
	if ident == nil {
		return "", false
	}
	callee, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok || callee.Pkg() == nil || callee.Pkg().Name() != "assert" {
		return "", false
	}
	return callee.Name(), true
}

// reportSplitAssertion fires TS-S07 when an assert.Ok or assert.Invariant
// call's condition argument contains &&.
func reportSplitAssertion(pass *analysis.Pass, call *ast.CallExpr) {
	name, ok := calleeName(pass, call)
	if !ok {
		return
	}
	condIndex := -1
	if name == "Ok" {
		condIndex = 0
	}
	if name == "Invariant" {
		condIndex = 1
	}
	if condIndex < 0 || condIndex >= len(call.Args) {
		return
	}
	cond := call.Args[condIndex]
	if !scanLogic(cond).hasAnd {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      cond.Pos(),
		Category: "TS-S07",
		Message: "TS-S07: assert." + name + " condition combines checks with && — split into " +
			"two assert." + name + " calls (one check each) so the failure names which half broke",
	})
}

// closedSetPackage reports whether named's own package declares at least
// one constant of that exact type — TS-S08's definition of a closed set.
func closedSetPackage(named *types.Named) bool {
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return false
	}
	for _, name := range pkg.Scope().Names() {
		constant, ok := pkg.Scope().Lookup(name).(*types.Const)
		if ok && types.Identical(constant.Type(), named) {
			return true
		}
	}
	return false
}

// defaultCase returns switchStmt's default clause, or nil when it has none.
func defaultCase(switchStmt *ast.SwitchStmt) *ast.CaseClause {
	for _, stmt := range switchStmt.Body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if ok && clause.List == nil {
			return clause
		}
	}
	return nil
}

// endsInUnreachable reports whether clause's last statement is a call to
// assert.Unreachable.
func endsInUnreachable(pass *analysis.Pass, clause *ast.CaseClause) bool {
	if len(clause.Body) == 0 {
		return false
	}
	last, ok := clause.Body[len(clause.Body)-1].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := last.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	name, ok := calleeName(pass, call)
	return ok && name == "Unreachable"
}

// reportClosedSwitch fires TS-S08 when switchStmt tags a closed-set named
// type and either has no default clause or its default does not end in
// assert.Unreachable. Type switches, tagless switches, and switches over
// plain basic types or open sets stay silent.
func reportClosedSwitch(pass *analysis.Pass, switchStmt *ast.SwitchStmt) {
	if switchStmt.Tag == nil {
		return
	}
	named, ok := pass.TypesInfo.TypeOf(switchStmt.Tag).(*types.Named)
	if !ok {
		return
	}
	basic, ok := named.Underlying().(*types.Basic)
	if !ok || basic.Info()&(types.IsInteger|types.IsString) == 0 {
		return
	}
	if !closedSetPackage(named) {
		return
	}
	typeName := named.Obj().Name()
	clause := defaultCase(switchStmt)
	if clause == nil {
		pass.Report(analysis.Diagnostic{
			Pos:      switchStmt.Pos(),
			Category: "TS-S08",
			Message: "TS-S08: switch over closed set " + typeName + " has no default arm — add " +
				"`default: assert.Unreachable(\"" + typeName + ": unhandled value\")` so a new " +
				"constant fails loudly",
		})
		return
	}
	if !endsInUnreachable(pass, clause) {
		pass.Report(analysis.Diagnostic{
			Pos:      clause.Case,
			Category: "TS-S08",
			Message: "TS-S08: switch over closed set " + typeName + "'s default arm does not " +
				"end in assert.Unreachable — make assert.Unreachable(...) the default arm's last " +
				"statement so a new constant fails loudly",
		})
	}
}
