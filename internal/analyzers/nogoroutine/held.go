package nogoroutine

import (
	"go/ast"
	"go/token"
	"go/types"
	"math"
)

const (
	heldLocalsMax   = 4
	heldLiteralsMax = 255
)

const (
	_ = uint(heldLocalsMax - 1)
	_ = uint(math.MaxUint8 - heldLiteralsMax)
)

type heldLiterals [heldLocalsMax]uint8

type followedLocal struct {
	local    *types.Var
	literals []*ast.FuncLit
}

type pathLiterals struct {
	followed []followedLocal
	held     heldLiterals
}

func (o *owners) literalAt(expr ast.Expr, path pathLiterals) *ast.FuncLit {
	if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
		local := o.index.Object(ident)
		for i, followed := range path.followed {
			if local == nil || followed.local != local {
				continue
			}
			if path.held[i] == 0 {
				return nil
			}
			return followed.literals[path.held[i]-1]
		}
	}
	return o.literalOf(expr)
}

func (o *owners) reassignedLiterals(fn function) []followedLocal {
	if fn.decl == nil {
		return nil
	}
	var followed []followedLocal
	ast.Inspect(fn.body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok || len(followed) == heldLocalsMax {
			return false
		}
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		local, defined := o.pass.TypesInfo.Defs[ident].(*types.Var)
		if !defined {
			return true
		}
		if literals, ok := o.followedLiterals(fn, local); ok {
			followed = append(followed, followedLocal{local: local, literals: literals})
		}
		return true
	})
	return followed
}

func (o *owners) followedLiterals(fn function, local *types.Var) ([]*ast.FuncLit, bool) {
	if _, isFunc := local.Type().Underlying().(*types.Signature); !isFunc {
		return nil, false
	}
	if o.index.IsParameter(local) || o.rebound(local, fn.decl) {
		return nil, false
	}
	defs := o.index.Definitions(local, fn.decl)
	if len(defs) < 2 || len(defs) > heldLiteralsMax {
		return nil, false
	}
	if !o.assignedIn(local, fn.decl, fn.body, defs) {
		return nil, false
	}
	var literals []*ast.FuncLit
	for _, def := range defs {
		if literal, ok := ast.Unparen(def).(*ast.FuncLit); ok {
			literals = append(literals, literal)
		}
	}
	return literals, true
}

func (o *owners) assignedLiterals(
	node ast.Node, followed []followedLocal, held heldLiterals,
) heldLiterals {
	for i, local := range followed {
		for _, value := range o.assignments(node, local.local) {
			held[i] = 0
			for at, literal := range local.literals {
				if ast.Unparen(value) == literal {
					held[i] = uint8(at + 1)
				}
			}
		}
	}
	return held
}

func calledInPlace(decl *ast.FuncDecl, body *ast.BlockStmt, use ast.Expr) ast.Node {
	path := pathTo(decl.Body, use.Pos())
	top := indexOf(path, body)
	if top < 0 {
		return nil
	}
	var site ast.Node = use
	for i := len(path) - 1; i > top; i-- {
		literal, ok := path[i].(*ast.FuncLit)
		if !ok {
			continue
		}
		run := callOfLiteral(path, i, literal)
		if run.call == nil || startedLater(run) {
			return nil
		}
		site = run.call
	}
	return site
}

func deferredIn(decl *ast.FuncDecl, body *ast.BlockStmt, def ast.Expr) bool {
	path := pathTo(decl.Body, def.Pos())
	top := indexOf(path, body)
	if top < 0 {
		return false
	}
	for i := top + 1; i < len(path); i++ {
		literal, ok := path[i].(*ast.FuncLit)
		if !ok {
			continue
		}
		run := callOfLiteral(path, i, literal)
		deferred, isDefer := run.parent.(*ast.DeferStmt)
		return run.call != nil && isDefer && deferred.Call == run.call
	}
	return false
}

type literalCall struct {
	call   *ast.CallExpr
	parent ast.Node
}

func callOfLiteral(path []ast.Node, i int, literal *ast.FuncLit) literalCall {
	at := i - 1
	for at > 0 {
		if _, paren := path[at].(*ast.ParenExpr); !paren {
			break
		}
		at--
	}
	call, ok := path[at].(*ast.CallExpr)
	if !ok || at == 0 || ast.Unparen(call.Fun) != literal {
		return literalCall{}
	}
	return literalCall{call: call, parent: path[at-1]}
}

func startedLater(run literalCall) bool {
	switch stmt := run.parent.(type) {
	case *ast.DeferStmt:
		return stmt.Call == run.call
	case *ast.GoStmt:
		return stmt.Call == run.call
	}
	return false
}

func pathTo(root ast.Node, pos token.Pos) []ast.Node {
	var path []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil || pos < node.Pos() || node.End() <= pos {
			return false
		}
		path = append(path, node)
		return true
	})
	return path
}

func indexOf(path []ast.Node, node ast.Node) int {
	for i, step := range path {
		if step == node {
			return i
		}
	}
	return -1
}
