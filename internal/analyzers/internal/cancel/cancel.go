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

	"github.com/kapetan-io/tiger/internal/analyzers/internal/counter"
)

// Parameters collects every function parameter declared in decl.
func Parameters(info *types.Info, decl ast.Node) map[*types.Var]bool {
	found := map[*types.Var]bool{}
	ast.Inspect(decl, func(node ast.Node) bool {
		var signature *ast.FuncType
		switch fn := node.(type) {
		case *ast.FuncDecl:
			signature = fn.Type
		case *ast.FuncLit:
			signature = fn.Type
		default:
			return true
		}
		for _, field := range signature.Params.List {
			for _, name := range field.Names {
				if v, ok := info.Defs[name].(*types.Var); ok {
					found[v] = true
				}
			}
		}
		return true
	})
	return found
}

// Waits reports whether a top-level && term of cond is ctx.Err() == nil on
// one of the parameters.
func Waits(info *types.Info, params map[*types.Var]bool, cond ast.Expr) bool {
	for _, term := range counter.Conjuncts(cond) {
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
		if ok && IsNil(info, other) && errOnParameter(info, params, call) {
			return true
		}
	}
	return false
}

// IsNil reports whether expr is the predeclared nil.
func IsNil(info *types.Info, expr ast.Expr) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return false
	}
	_, isNilObject := info.Uses[ident].(*types.Nil)
	return isNilObject
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
