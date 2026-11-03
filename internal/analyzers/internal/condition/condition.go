// Package condition reads the shape of the expressions the loop rules look
// at: the terms of a boolean condition joined by && or by ||, which comparison
// reads the same with its operands swapped, whether an expression is the
// predeclared nil, and which function a call names once the type arguments of
// an instantiation are set aside.
package condition

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/kapetan-io/tiger/assert"
)

const termsMax = 1 << 16

const _ = uint(1<<20 - termsMax)

func Conjuncts(expr ast.Expr) []ast.Expr {
	return split(expr, token.LAND)
}

func Disjuncts(expr ast.Expr) []ast.Expr {
	return split(expr, token.LOR)
}

func split(expr ast.Expr, op token.Token) []ast.Expr {
	pending := []ast.Expr{expr}
	var terms []ast.Expr
	for steps := 0; len(pending) > 0 && steps < termsMax; steps++ {
		last := len(pending) - 1
		next := ast.Unparen(pending[last])
		pending = pending[:last]
		binary, ok := next.(*ast.BinaryExpr)
		if ok && binary.Op == op {
			pending = append(pending, binary.Y, binary.X)
			continue
		}
		terms = append(terms, next)
	}
	assert.Ok(len(pending) == 0, "condition has more than termsMax terms")
	return terms
}

var swapped = map[token.Token]token.Token{
	token.LSS: token.GTR, token.LEQ: token.GEQ, token.GTR: token.LSS, token.GEQ: token.LEQ,
}

func Mirror(op token.Token) (token.Token, bool) {
	mirrored, ok := swapped[op]
	return mirrored, ok
}

func IsNil(info *types.Info, expr ast.Expr) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return false
	}
	_, isNilObject := info.Uses[ident].(*types.Nil)
	return isNilObject
}

func Uninstantiated(fun ast.Expr) ast.Expr {
	fun = ast.Unparen(fun)
	switch instantiated := fun.(type) {
	case *ast.IndexExpr:
		return ast.Unparen(instantiated.X)
	case *ast.IndexListExpr:
		return ast.Unparen(instantiated.X)
	}
	return fun
}
