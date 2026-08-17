// Package testdoc enforces TS-T06: test functions have a doc comment.
//
// Every FuncDecl in a _test.go file shaped like a Go test function — named
// Test or TestXxx, taking a single *testing.T parameter — needs a doc
// comment. Benchmarks, fuzz targets, and helpers are out of wave-1 scope;
// only the Test-function shape is checked.
package testdoc

import (
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-T06: test functions have a doc comment.
var Analyzer = &analysis.Analyzer{
	Name: "testdoc",
	Doc:  "TS-T06: test functions have a doc comment.",
	Run:  run,
}

// testNamePrefix matches every TestXxx name except the exact name "Test",
// which testNameExact covers separately.
var testNamePrefix = regexp.MustCompile(`^Test\p{Lu}`)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if !strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !isTestFunc(pass, function) {
				continue
			}
			if function.Doc == nil {
				pass.Report(analysis.Diagnostic{
					Pos:      function.Pos(),
					Category: "TS-T06",
					Message: "TS-T06: " + function.Name.Name + " has no doc comment — a reader " +
						"should be able to skip a test or dive into it without " +
						"reverse-engineering the setup; add a doc comment describing " +
						"what the test proves",
				})
			}
		}
	}
	return nil, nil
}

// isTestFunc reports whether function is shaped like a Go test function: a
// name of Test or TestXxx, taking a single *testing.T parameter.
func isTestFunc(pass *analysis.Pass, function *ast.FuncDecl) bool {
	name := function.Name.Name
	if name != "Test" && !testNamePrefix.MatchString(name) {
		return false
	}
	if function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return false
	}
	return isTestingT(pass.TypesInfo.TypeOf(function.Type.Params.List[0].Type))
}

// isTestingT reports whether target is *testing.T.
func isTestingT(target types.Type) bool {
	pointer, ok := target.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := pointer.Elem().(*types.Named)
	if !ok {
		return false
	}
	symbol := named.Obj()
	return symbol.Pkg() != nil && symbol.Pkg().Path() == "testing" && symbol.Name() == "T"
}
