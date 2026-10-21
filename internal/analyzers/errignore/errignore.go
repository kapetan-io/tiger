// Package errignore enforces TS-E02: an error discarded with _ is a finding.
//
// No comment clears it, because tiger cannot tell a reason from `// x`
// (ADR-0031). Only a change to the code does.
//
// Discards inside the function literal a defer statement calls, or a
// function literal passed to a testing Cleanup method, are exempt
// (ADR-0017). Arguments evaluated at the defer or Cleanup call run
// immediately and are still checked. Closing something on the way out has
// no better answer than dropping the error. Cleanup that is not deferred
// is still a finding.
package errignore

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-E02: an error discarded with _ is a finding.
var Analyzer = &analysis.Analyzer{
	Name: "errignore",
	Doc:  "TS-E02: discarding an error with _ is a finding, whatever comment sits next to it",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		exempt := map[*ast.FuncLit]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.DeferStmt:
				if lit, ok := stmt.Call.Fun.(*ast.FuncLit); ok {
					exempt[lit] = true
				}
			case *ast.CallExpr:
				if isTestingCleanup(pass, stmt) {
					for _, arg := range stmt.Args {
						if lit, ok := arg.(*ast.FuncLit); ok {
							exempt[lit] = true
						}
					}
				}
			case *ast.FuncLit:
				return !exempt[stmt]
			case *ast.AssignStmt:
				inspectAssign(pass, stmt)
			}
			return true
		})
	}
	return nil, nil
}

// isTestingCleanup reports whether call is a Cleanup method call on one of
// testing's types.
func isTestingCleanup(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Cleanup" {
		return false
	}
	obj := pass.TypesInfo.ObjectOf(sel.Sel)
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "testing"
}

// inspectAssign reports every blank left-hand side of assign whose matching
// right-hand-side result is error-typed.
func inspectAssign(pass *analysis.Pass, assign *ast.AssignStmt) {
	results := resultTypes(pass, assign)
	if len(results) != len(assign.Lhs) {
		return
	}
	for i := 0; i < len(results); i++ {
		blank, ok := assign.Lhs[i].(*ast.Ident)
		if !ok || blank.Name != "_" {
			continue
		}
		if !isError(results[i]) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      assign.Lhs[i].Pos(),
			Category: "TS-E02",
			Message: "TS-E02: this error is discarded with _, so a failure here goes unnoticed " +
				"— tiger expected the error handled, and a comment does not change that — " +
				"return it, check it, or log it where the caller can see it",
		})
	}
}

// resultTypes returns one type per left-hand-side position of assign. A
// single call on the right assigned to more than one name yields the
// call's result tuple; otherwise each right-hand-side expression is typed
// on its own.
func resultTypes(pass *analysis.Pass, assign *ast.AssignStmt) []types.Type {
	if len(assign.Rhs) == 1 && len(assign.Lhs) > 1 {
		tuple, ok := pass.TypesInfo.TypeOf(assign.Rhs[0]).(*types.Tuple)
		if !ok {
			return nil
		}
		results := make([]types.Type, tuple.Len())
		for i := 0; i < tuple.Len(); i++ {
			results[i] = tuple.At(i).Type()
		}
		return results
	}
	results := make([]types.Type, len(assign.Rhs))
	for i, rhs := range assign.Rhs {
		results[i] = pass.TypesInfo.TypeOf(rhs)
	}
	return results
}

// isError reports whether t is the predeclared error interface.
func isError(t types.Type) bool {
	if t == nil {
		return false
	}
	return t == types.Universe.Lookup("error").Type()
}
