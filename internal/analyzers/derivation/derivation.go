// Package derivation enforces TS-S22: a limit constant's stated derivation
// evaluates to its value.
//
// Judgment does not disappear when a formula moves from a reviewer's head
// into a comment; it moves into the comment. This analyzer does not decide
// whether a constant's value is correct — it decides whether the
// expression written next to the constant still computes that value. A
// doc comment line of the form "Name = expression" is parsed as a Go
// expression, every identifier in it is resolved against the package
// scope, and the result is evaluated with go/constant arithmetic and
// compared against the constant's own value. Anything the analyzer cannot
// parse or resolve is left alone as prose.
package derivation

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-S22: a limit constant's stated derivation evaluates
// to its value.
var Analyzer = &analysis.Analyzer{
	Name: "derivation",
	Doc:  "TS-S22: a limit constant's stated derivation evaluates to its value",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		checkFile(pass, file)
	}
	return nil, nil
}

func checkFile(pass *analysis.Pass, file *ast.File) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			constSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			checkSpec(pass, genDecl, constSpec)
		}
	}
}

// checkSpec resolves the doc comment that governs one ValueSpec, its own
// if present, otherwise the enclosing GenDecl's, and checks every constant
// name it declares.
func checkSpec(pass *analysis.Pass, genDecl *ast.GenDecl, spec *ast.ValueSpec) {
	doc := spec.Doc
	if doc == nil {
		doc = genDecl.Doc
	}
	if doc == nil {
		return
	}
	for _, name := range spec.Names {
		if name.Name == "_" {
			continue
		}
		checkConstant(pass, name, doc)
	}
}

func checkConstant(pass *analysis.Pass, name *ast.Ident, doc *ast.CommentGroup) {
	exprText, ok := derivationLine(doc, name.Name)
	if !ok {
		return
	}
	parsed, err := parser.ParseExpr(exprText)
	if err != nil {
		return // prose that happens to start with "Name =" but is not an expression
	}
	derived, ok := evalConst(pass, parsed)
	if !ok {
		return // an identifier in the expression did not resolve to a package constant
	}
	resolved, ok := pass.TypesInfo.Defs[name].(*types.Const)
	if !ok {
		return
	}
	actual := resolved.Val()
	if constant.Compare(actual, token.EQL, derived) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      name.Pos(),
		Category: "TS-S22",
		Message: "TS-S22: " + name.Name + " is " + actual.String() + " but its stated derivation " +
			exprText + " evaluates to " + derived.String() +
			" — the sketch went stale; recompute the constant or fix the derivation",
	})
}

// derivationLine scans a doc comment for a line of the form "Name =
// expression", optionally ending in the period godot adds, and returns the
// expression text.
func derivationLine(doc *ast.CommentGroup, name string) (string, bool) {
	prefix := name + " ="
	for _, line := range strings.Split(doc.Text(), "\n") {
		trimmed := strings.TrimSuffix(strings.TrimSpace(line), ".")
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), true
	}
	return "", false
}

// evalConst evaluates a derivation expression using go/constant arithmetic,
// resolving every identifier against the package scope. It reports ok=false
// for anything it cannot evaluate: an unresolved identifier or an operator
// outside the supported set.
func evalConst(pass *analysis.Pass, expr ast.Expr) (constant.Value, bool) {
	switch node := expr.(type) {
	case *ast.ParenExpr:
		return evalConst(pass, node.X)
	case *ast.BasicLit:
		return constant.MakeFromLiteral(node.Value, node.Kind, 0), true
	case *ast.Ident:
		return constFromScope(pass, node.Name)
	case *ast.UnaryExpr:
		return evalUnary(pass, node)
	case *ast.BinaryExpr:
		return evalBinary(pass, node)
	default:
		return nil, false
	}
}

func constFromScope(pass *analysis.Pass, name string) (constant.Value, bool) {
	resolved, ok := pass.Pkg.Scope().Lookup(name).(*types.Const)
	if !ok {
		return nil, false
	}
	return resolved.Val(), true
}

func evalUnary(pass *analysis.Pass, node *ast.UnaryExpr) (constant.Value, bool) {
	if node.Op != token.SUB {
		return nil, false
	}
	operand, ok := evalConst(pass, node.X)
	if !ok {
		return nil, false
	}
	return constant.UnaryOp(token.SUB, operand, 0), true
}

func evalBinary(pass *analysis.Pass, node *ast.BinaryExpr) (constant.Value, bool) {
	left, ok := evalConst(pass, node.X)
	if !ok {
		return nil, false
	}
	right, ok := evalConst(pass, node.Y)
	if !ok {
		return nil, false
	}
	return binaryOp(node.Op, operands{left: left, right: right})
}

// operands holds a binary expression's two already-evaluated sides so
// binaryOp takes one struct parameter instead of two adjacent values of
// the same type.
type operands struct {
	left  constant.Value
	right constant.Value
}

// binaryOp evaluates one supported operator. Division uses
// token.QUO_ASSIGN rather than token.QUO: go/constant's QUO on two Int
// operands produces a Float, and QUO_ASSIGN is the documented way to force
// Go's truncating integer division.
func binaryOp(op token.Token, pair operands) (constant.Value, bool) {
	if op == token.QUO {
		if constant.Sign(pair.right) == 0 {
			return nil, false
		}
		return constant.BinaryOp(pair.left, token.QUO_ASSIGN, pair.right), true
	}
	if op == token.REM {
		if constant.Sign(pair.right) == 0 {
			return nil, false
		}
		return constant.BinaryOp(pair.left, op, pair.right), true
	}
	if op == token.SHL || op == token.SHR {
		return shiftConst(pair, op)
	}
	if arithmetic(op) {
		return constant.BinaryOp(pair.left, op, pair.right), true
	}
	return nil, false
}

// arithmetic reports whether op is one of the plain binary operators a
// derivation may use.
func arithmetic(op token.Token) bool {
	return op == token.ADD || op == token.SUB || op == token.MUL ||
		op == token.AND || op == token.OR || op == token.XOR
}

func shiftConst(pair operands, op token.Token) (constant.Value, bool) {
	shift, exact := constant.Int64Val(pair.right)
	if !exact || shift < 0 {
		return nil, false
	}
	return constant.Shift(pair.left, op, uint(shift)), true
}
