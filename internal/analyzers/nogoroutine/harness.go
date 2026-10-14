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

// checkHarness reports TS-C03 once, at the first goroutine start, for a
// package whose non-test files start goroutines when no test file of the
// package has a TestMain that uses goleak. The package's tests may be in the
// package itself or in its _test package, so the check reads every _test.go
// file in the package's directory.
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

// hasLeakCheck reports whether any _test.go file in dir declares a TestMain
// that calls into goleak.
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

// mainUsesGoleak reports whether file imports goleak and its TestMain refers
// to the import.
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

// refers reports whether body selects from the package imported as local.
func refers(body *ast.BlockStmt, local string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok {
			if ident, isIdent := selector.X.(*ast.Ident); isIdent && ident.Name == local {
				found = true
			}
		}
		return !found
	})
	return found
}
