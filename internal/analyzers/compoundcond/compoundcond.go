// Package compoundcond enforces TS-S06, TS-S07, and TS-S08: simple
// conditions, split assertions, asserted default arms. TS-S08 exempts a
// type declared //tiger:openenum from the assert.Unreachable requirement:
// the set is deliberately extensible, so a plain default arm is the
// compliant catch-all for values the wire can still send.
package compoundcond

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/assertpkg"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/attach"
	"github.com/kapetan-io/tiger/internal/worklist"
)

var Analyzer = &analysis.Analyzer{
	Name: "compoundcond",
	Doc:  "TS-S06, TS-S07, and TS-S08: simple conditions, split assertions, asserted default arms.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	openEnums := collectOpenEnums(pass)
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
				reportClosedSwitch(pass, switchStmt, openEnums)
			}
			return true
		})
	}
	return nil, nil
}

func collectOpenEnums(pass *analysis.Pass) map[*types.Named]bool {
	marked := map[*types.Named]bool{}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				_, onGenDecl := attach.Directive(pass.Fset, file, genDecl, "openenum")
				_, onTypeSpec := attach.Directive(pass.Fset, file, typeSpec, "openenum")
				if !onGenDecl && !onTypeSpec {
					continue
				}
				object, ok := pass.TypesInfo.Defs[typeSpec.Name].(*types.TypeName)
				if !ok {
					continue
				}
				named, ok := object.Type().(*types.Named)
				if !ok {
					continue
				}
				marked[named] = true
			}
		}
	}
	return marked
}

type logicShape struct {
	hasAnd bool
	hasOr  bool
}

func scanLogic(expr ast.Expr) logicShape {
	shape := logicShape{}
	work := []ast.Expr{expr}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		switch at := work[i].(type) {
		case *ast.ParenExpr:
			work = append(work, at.X)
		case *ast.UnaryExpr:
			if at.Op == token.NOT {
				work = append(work, at.X)
			}
		case *ast.BinaryExpr:
			if at.Op == token.LAND {
				shape.hasAnd = true
				work = append(work, at.X, at.Y)
			}
			if at.Op == token.LOR {
				shape.hasOr = true
				work = append(work, at.X, at.Y)
			}
		}
	}
	assert.Ok(i == len(work), "scanLogic: worklist reached worklist.NodesMax")
	return shape
}

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
		Message: "TS-S06: this condition mixes && and ||, so the reader has to work out " +
			"precedence — split it into nested if/else with one operator per condition",
	})
}

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
	if !ok || callee.Pkg() == nil || !assertpkg.Is(callee.Pkg().Path()) {
		return "", false
	}
	return callee.Name(), true
}

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
		Message: "TS-S07: this assert." + name + " checks two things joined by && — split it " +
			"into one assert." + name + " per check so a failure names which one broke",
	})
}

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

func defaultCase(switchStmt *ast.SwitchStmt) *ast.CaseClause {
	for _, stmt := range switchStmt.Body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if ok && clause.List == nil {
			return clause
		}
	}
	return nil
}

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

func reportClosedSwitch(
	pass *analysis.Pass,
	switchStmt *ast.SwitchStmt,
	openEnums map[*types.Named]bool,
) {
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
	open := openEnums[named]
	if clause == nil {
		if open {
			pass.Report(analysis.Diagnostic{
				Pos:      switchStmt.Pos(),
				Category: "TS-S08",
				Message: "TS-S08: this switch over " + typeName + " has no default arm, and " +
					typeName + " can hold values from outside the package — add a default arm " +
					"that handles them",
			})
			return
		}
		pass.Report(analysis.Diagnostic{
			Pos:      switchStmt.Pos(),
			Category: "TS-S08",
			Message: "TS-S08: this switch over " + typeName + " has no default arm — add " +
				"default: assert.Unreachable(\"" + typeName + ": unhandled value\") so a new " +
				typeName + " constant fails loudly",
		})
		return
	}
	if open {
		return
	}
	if !endsInUnreachable(pass, clause) {
		pass.Report(analysis.Diagnostic{
			Pos:      clause.Case,
			Category: "TS-S08",
			Message: "TS-S08: this switch over " + typeName + " has a default arm that doesn't " +
				"end in assert.Unreachable — make assert.Unreachable(\"" + typeName +
				": unhandled value\") its last statement so a new " + typeName +
				" constant fails loudly",
		})
	}
}
