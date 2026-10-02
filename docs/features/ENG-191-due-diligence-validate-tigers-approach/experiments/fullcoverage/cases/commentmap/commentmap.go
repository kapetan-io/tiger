// Package commentmap reproduces go1.26.2 src/go/ast/commentmap.go:301 (CommentMap.String): the key is
// the ast.Node interface (pointer dynamic types). Pos/End do not identify a node: an
// ExprStmt and the CallExpr it wraps share both.
package commentmap

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strings"

	"fullcoverage/cases/variant"
)

func line(buf *strings.Builder, node ast.Node, cmap ast.CommentMap) {
	s := fmt.Sprintf("%T", node)
	if ident, ok := node.(*ast.Ident); ok {
		s = ident.Name
	}
	fmt.Fprintf(buf, "\t%20s:  %s\n", s, cmap[node][0].Text())
}

// Original is the shipped shape (the %p column is dropped so outputs compare across runs).
func Original(cmap ast.CommentMap) string {
	var nodes []ast.Node
	for node := range cmap {
		nodes = append(nodes, node)
	}
	slices.SortFunc(nodes, func(a, b ast.Node) int {
		r := cmp.Compare(a.Pos(), b.Pos())
		if r != 0 {
			return r
		}
		return cmp.Compare(a.End(), b.End())
	})

	var buf strings.Builder
	for _, node := range nodes {
		line(&buf, node, cmap)
	}
	return buf.String()
}

// S2 is SortedFunc with a key-only comparator: reads only a and b, escapes TS-T02, and
// still leaks map order because Pos/End is a partial projection of the key.
func S2(cmap ast.CommentMap) string {
	byPos := func(a, b ast.Node) int {
		return cmp.Or(cmp.Compare(a.Pos(), b.Pos()), cmp.Compare(a.End(), b.End()))
	}
	var buf strings.Builder
	for _, node := range slices.SortedFunc(maps.Keys(cmap), byPos) {
		line(&buf, node, cmap)
	}
	return buf.String()
}

// S2Type adds the dynamic type name; key-only and deterministic here, still not injective
// (two *ast.Ident at token.NoPos tie).
func S2Type(cmap ast.CommentMap) string {
	byPos := func(a, b ast.Node) int {
		return cmp.Or(cmp.Compare(a.Pos(), b.Pos()), cmp.Compare(a.End(), b.End()),
			strings.Compare(fmt.Sprintf("%T", a), fmt.Sprintf("%T", b)))
	}
	var buf strings.Builder
	for _, node := range slices.SortedFunc(maps.Keys(cmap), byPos) {
		line(&buf, node, cmap)
	}
	return buf.String()
}

// S4 keeps the collect loop and adds the type name to the comparator.
func S4(cmap ast.CommentMap) string {
	var nodes []ast.Node
	for node := range cmap {
		nodes = append(nodes, node)
	}
	slices.SortFunc(nodes, func(a, b ast.Node) int {
		return cmp.Or(cmp.Compare(a.Pos(), b.Pos()), cmp.Compare(a.End(), b.End()),
			strings.Compare(fmt.Sprintf("%T", a), fmt.Sprintf("%T", b)))
	})

	var buf strings.Builder
	for _, node := range nodes {
		line(&buf, node, cmap)
	}
	return buf.String()
}

// S5 keeps the order elsewhere: walk the file, which visits nodes in a fixed order, and
// print the ones the map holds. No map range at all.
func S5(file *ast.File, cmap ast.CommentMap) string {
	var buf strings.Builder
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := cmap[node]; ok {
			line(&buf, node, cmap)
		}
		return true
	})
	return buf.String()
}

const src = `package p

func f() {
	g() // c1
	h()
	k() // c3
}
`

func fixture() (*ast.File, ast.CommentMap) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	cmap := ast.NewCommentMap(fset, file, file.Comments)
	// Attach a second comment to the CallExpr inside each commented ExprStmt: same Pos/End.
	for _, stmt := range file.Decls[0].(*ast.FuncDecl).Body.List {
		es := stmt.(*ast.ExprStmt)
		cmap[es.X] = []*ast.CommentGroup{{List: []*ast.Comment{{Text: "// call of " + fmt.Sprint(es.X.(*ast.CallExpr).Fun)}}}}
		if _, ok := cmap[es]; !ok {
			cmap[es] = []*ast.CommentGroup{{List: []*ast.Comment{{Text: "// stmt"}}}}
		}
	}
	return file, cmap
}

func render(f func(ast.CommentMap) string) func() string {
	return func() string {
		_, cmap := fixture()
		return f(cmap)
	}
}

var Variants = []variant.Variant{
	{Name: "Original", Run: render(Original)},
	{Name: "S2", Run: render(S2)},
	{Name: "S2Type", Run: render(S2Type)},
	{Name: "S4", Run: render(S4)},
	{Name: "S5", Run: func() string { return S5(fixture()) }},
}
