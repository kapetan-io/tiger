// Package errignore enforces TS-E02: discarding a result with _ requires a
// justification comment.
//
// The check is a shape check, not a truth check: it verifies a comment is
// present, either trailing the statement or ending on the line directly
// above it, and never judges what the comment says. A vacuous comment
// satisfies the rule; only a human review catches a false claim.
package errignore

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-E02: discarding a result with _ requires a
// justification comment.
var Analyzer = &analysis.Analyzer{
	Name: "errignore",
	Doc:  "TS-E02: discarding a result with _ requires a justification comment",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			inspectAssign(pass, file, assign)
			return true
		})
	}
	return nil, nil
}

// inspectAssign reports every blank left-hand side of assign whose matching
// right-hand-side result is error-typed and carries no justification
// comment.
func inspectAssign(pass *analysis.Pass, file *ast.File, assign *ast.AssignStmt) {
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
		if hasJustification(pass, file, assign) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      assign.Lhs[i].Pos(),
			Category: "TS-E02",
			Message: "TS-E02: discarding this error silently hides a failure — say why the " +
				"discard is safe in a trailing comment, or a comment on the line above, or " +
				"handle the error",
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

// hasJustification reports whether assign carries a comment either
// trailing it on the same line or ending on the line directly above it.
func hasJustification(pass *analysis.Pass, file *ast.File, assign *ast.AssignStmt) bool {
	statementLine := pass.Fset.Position(assign.Pos()).Line
	endLine := pass.Fset.Position(assign.End()).Line
	for _, group := range file.Comments {
		if group.Pos() > assign.End() && pass.Fset.Position(group.Pos()).Line == endLine {
			return true
		}
		if pass.Fset.Position(group.End()).Line == statementLine-1 {
			return true
		}
	}
	return false
}
