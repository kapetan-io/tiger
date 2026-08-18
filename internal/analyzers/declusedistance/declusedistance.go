// Package declusedistance enforces TS-S13: variables are declared at the
// point of first use.
//
// A declaration and a use that sit far apart force the reader to hold the
// variable in their head across everything in between. The heuristic here
// is textual distance: more than maxDistance lines between a statement-level
// declaration and the first reference to it, resolved through
// pass.TypesInfo so renaming or shadowing never fools the match. The
// deferdistance lessons apply directly: a FuncLit is a frame boundary, so a
// use inside a nested closure counts as one use at the closure's own
// position, and distance is never measured across frames — a variable
// declared in an outer frame and first touched inside a closure measures to
// the closure's line, not the line of the touch inside it. This is a
// deliberate blind spot, not a bug: see the ts-s13 known-miss case.
package declusedistance

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// maxDistance is the spec's calibration constant: TS-S13 fires only when a
// declaration and its first use sit more than this many lines apart.
const maxDistance = 10

// Analyzer enforces TS-S13: variables are declared at the point of first
// use.
var Analyzer = &analysis.Analyzer{
	Name: "declusedistance",
	Doc:  "TS-S13: variables are declared at the point of first use.",
	Run:  run,
}

// decl tracks a single statement-level declaration awaiting its first use.
type decl struct {
	name    string
	obj     types.Object
	pos     token.Pos
	line    int
	used    bool
	useLine int
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		skip := initClauses(file)
		ast.Inspect(file, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			checkFrame(pass, fn.Body, skip)
			return true
		})
	}
	return nil, nil
}

// initClauses collects the identity of every for/if/switch init-clause
// statement, plus a type switch's guard assignment, in file. A variable
// declared there sits at its point of use by construction and is exempt
// from tracking.
func initClauses(file *ast.File) map[ast.Stmt]bool {
	skip := map[ast.Stmt]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.ForStmt:
			markSkip(skip, n.Init)
		case *ast.IfStmt:
			markSkip(skip, n.Init)
		case *ast.SwitchStmt:
			markSkip(skip, n.Init)
		case *ast.TypeSwitchStmt:
			markSkip(skip, n.Init)
			markSkip(skip, n.Assign)
		}
		return true
	})
	return skip
}

func markSkip(skip map[ast.Stmt]bool, stmt ast.Stmt) {
	if stmt != nil {
		skip[stmt] = true
	}
}

// checkFrame walks body — a function or closure's block — collecting
// statement-level declarations and their first use, then reports the ones
// separated by more than maxDistance lines. A FuncLit encountered along the
// way is a frame boundary: its entire subtree is checked once, against
// this frame's still-unused declarations, as a single use positioned at
// the closure's own line; the closure is then walked again independently
// as its own frame for its own declarations.
func checkFrame(pass *analysis.Pass, body *ast.BlockStmt, skip map[ast.Stmt]bool) {
	decls := []*decl{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			line := pass.Fset.Position(n.Pos()).Line
			for _, d := range decls {
				if !d.used && referencesObject(pass, n, d.obj) {
					d.used = true
					d.useLine = line
				}
			}
			checkFrame(pass, n.Body, skip)
			return false
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE && !skip[n] {
				decls = append(decls, defineDecls(pass, n.Lhs)...)
			}
		case *ast.DeclStmt:
			decls = append(decls, varDecls(pass, n)...)
		case *ast.Ident:
			markUse(pass, decls, n)
		}
		return true
	})
	for _, d := range decls {
		if d.used && d.useLine-d.line > maxDistance {
			report(pass, d)
		}
	}
}

// defineDecls returns the new declarations a := statement's left-hand side
// introduces, skipping the blank identifier and any name that is a plain
// reuse of an existing variable (recognized because go/types records reuse
// in Uses, not Defs).
func defineDecls(pass *analysis.Pass, lhs []ast.Expr) []*decl {
	found := []*decl{}
	for _, expr := range lhs {
		ident, ok := expr.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		obj := pass.TypesInfo.Defs[ident]
		if obj == nil {
			continue
		}
		found = append(found, newDecl(pass, ident, obj))
	}
	return found
}

// varDecls returns the declarations a var DeclStmt introduces.
func varDecls(pass *analysis.Pass, declStmt *ast.DeclStmt) []*decl {
	genDecl, ok := declStmt.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.VAR {
		return nil
	}
	found := []*decl{}
	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, ident := range valueSpec.Names {
			if ident.Name == "_" {
				continue
			}
			obj := pass.TypesInfo.Defs[ident]
			if obj == nil {
				continue
			}
			found = append(found, newDecl(pass, ident, obj))
		}
	}
	return found
}

func newDecl(pass *analysis.Pass, ident *ast.Ident, obj types.Object) *decl {
	return &decl{
		name: ident.Name,
		obj:  obj,
		pos:  ident.Pos(),
		line: pass.Fset.Position(ident.Pos()).Line,
	}
}

// markUse records ident's position as the first use of whichever pending
// declaration it resolves to, identified by object identity rather than
// name.
func markUse(pass *analysis.Pass, decls []*decl, ident *ast.Ident) {
	obj := pass.TypesInfo.Uses[ident]
	if obj == nil {
		return
	}
	for _, d := range decls {
		if !d.used && d.obj == obj {
			d.used = true
			d.useLine = pass.Fset.Position(ident.Pos()).Line
			return
		}
	}
}

// referencesObject reports whether obj is referenced anywhere within node,
// however deeply nested — used to test a closure's entire subtree as a
// single unit against an outer frame's pending declarations.
func referencesObject(pass *analysis.Pass, node ast.Node, obj types.Object) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if ok && pass.TypesInfo.Uses[ident] == obj {
			found = true
			return false
		}
		return true
	})
	return found
}

func report(pass *analysis.Pass, d *decl) {
	pass.Report(analysis.Diagnostic{
		Pos:      d.pos,
		Category: "TS-S13",
		Message: fmt.Sprintf(
			"TS-S13: %s is declared %d lines before its first use — move the declaration "+
				"down next to where %s is first used",
			d.name, d.useLine-d.line, d.name,
		),
	})
}
