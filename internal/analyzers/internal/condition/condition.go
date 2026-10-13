// Package condition reads the shape of boolean conditions the loop rules look
// at: the terms joined by && or by ||, which comparison reads the same with
// its operands swapped, and whether an expression is the predeclared nil.
package condition

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/kapetan-io/tiger/assert"
)

// termsMax bounds how many nodes the flattening of one condition visits. No
// real condition has this many terms, but generated code can have thousands.
const termsMax = 1 << 16

const _ = uint(1<<20 - termsMax)

// Conjuncts splits expr on top-level && into its terms, looking through
// parentheses but not through ! or ||.
func Conjuncts(expr ast.Expr) []ast.Expr {
	return split(expr, token.LAND)
}

// Disjuncts splits expr on top-level || into its terms, looking through
// parentheses but not through ! or &&.
func Disjuncts(expr ast.Expr) []ast.Expr {
	return split(expr, token.LOR)
}

// split flattens a tree of op into its leaves, left to right.
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

// swapped maps a comparison to the one that reads the same with its operands
// swapped.
var swapped = map[token.Token]token.Token{
	token.LSS: token.GTR, token.LEQ: token.GEQ, token.GTR: token.LSS, token.GEQ: token.LEQ,
}

// Mirror returns the comparison that reads the same as op with its operands
// swapped, and reports whether op is an ordering comparison.
func Mirror(op token.Token) (token.Token, bool) {
	mirrored, ok := swapped[op]
	return mirrored, ok
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
