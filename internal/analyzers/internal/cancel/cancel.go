// Package cancel recognizes a loop that runs until its caller cancels a
// context: for ctx.Err() == nil, with ctx a context.Context parameter. Such a
// loop is a forever loop, like a select on ctx.Done(), so the termination
// rules leave it to the cancellation rules. A context built inside the
// function (context.Background(), context.TODO()) is never cancelled by a
// caller, so it does not qualify.
package cancel

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/condition"
)

// UntilCancelled reports whether a top-level && term of cond is
// ctx.Err() == nil on one of the parameters, which makes the loop run until the
// caller cancels.
func UntilCancelled(info *types.Info, params map[*types.Var]bool, cond ast.Expr) bool {
	for _, term := range condition.Conjuncts(cond) {
		binary, ok := term.(*ast.BinaryExpr)
		if !ok || binary.Op != token.EQL {
			continue
		}
		call, ok := ast.Unparen(binary.X).(*ast.CallExpr)
		other := binary.Y
		if !ok {
			call, ok = ast.Unparen(binary.Y).(*ast.CallExpr)
			other = binary.X
		}
		if ok && condition.IsNil(info, other) && errOnParameter(info, params, call) {
			return true
		}
	}
	return false
}

// errOnParameter reports whether call is Err() on a parameter of type
// context.Context.
func errOnParameter(info *types.Info, params map[*types.Var]bool, call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Err" {
		return false
	}
	receiver, ok := ast.Unparen(selector.X).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := info.Uses[receiver].(*types.Var)
	if !ok || !params[v] {
		return false
	}
	named, ok := v.Type().(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}
