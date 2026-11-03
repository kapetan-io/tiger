package nogoroutine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const goleakPath = `"go.uber.org/goleak"`

func checkHarness(pass *analysis.Pass, starts []ast.Node) {
	var first ast.Node
	for _, start := range starts {
		file := pass.Fset.PositionFor(start.Pos(), false).Filename
		if !strings.HasSuffix(file, "_test.go") {
			first = start
			break
		}
	}
	if first == nil {
		return
	}
	dir := filepath.Dir(pass.Fset.PositionFor(first.Pos(), false).Filename)
	if hasLeakCheck(dir) {
		return
	}
	pass.Report(analysis.Diagnostic{Pos: first.Pos(), Category: "TS-C03", Message: msgNoLeakCheck})
}

func hasLeakCheck(dir string) bool {
	names, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return false
	}
	for _, name := range names {
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			continue
		}
		if mainUsesGoleak(file) {
			return true
		}
	}
	return false
}

func mainUsesGoleak(file *ast.File) bool {
	local := ""
	for _, spec := range file.Imports {
		if spec.Path.Value != goleakPath {
			continue
		}
		local = "goleak"
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	if local == "" {
		return false
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "TestMain" || fn.Body == nil {
			continue
		}
		if refers(fn.Body, local) {
			return true
		}
	}
	return false
}

func refers(body *ast.BlockStmt, local string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if branch, ok := node.(*ast.IfStmt); ok && isFalse(branch.Cond) {
			if branch.Else != nil && refers(&ast.BlockStmt{List: []ast.Stmt{branch.Else}}, local) {
				found = true
			}
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return !found
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector || selector.Sel.Name != "VerifyTestMain" {
			return !found
		}
		if ident, isIdent := selector.X.(*ast.Ident); isIdent && ident.Name == local {
			found = true
		}
		return !found
	})
	return found
}

func isFalse(cond ast.Expr) bool {
	ident, ok := cond.(*ast.Ident)
	return ok && ident.Name == "false"
}
