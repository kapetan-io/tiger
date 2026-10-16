// Package testpackage enforces TS-T12: tests live in the external _test package.
//
// A _test.go file that declares the package under test (package foo) can reach
// every unexported name in it, which lets a test pin internals instead of
// behavior a caller sees. Only package foo_test is allowed. There is no
// exception for export_test.go: a file that re-exports internals is the same
// problem in a different file.
package testpackage

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-T12: tests live in the external _test package.
var Analyzer = &analysis.Analyzer{
	Name: "testpackage",
	Doc:  "TS-T12: tests live in the external _test package.",
	Run:  run,
}

const testSuffix = "_test"

func run(pass *analysis.Pass) (any, error) {
	// A package can itself be named foo_test, so the suffix alone cannot tell
	// internal test files from external ones. A name shared with a non-test file can.
	underTest := map[string]bool{}
	for _, file := range pass.Files {
		if !isTestFile(pass, file) {
			underTest[file.Name.Name] = true
		}
	}
	for _, file := range pass.Files {
		if !isTestFile(pass, file) {
			continue
		}
		name := file.Name.Name
		if strings.HasSuffix(name, testSuffix) && !underTest[name] {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      file.Name.Pos(),
			Category: "TS-T12",
			Message: "TS-T12: this test file declares package " + name + ", which gives it " +
				"every unexported name — declare package " + name +
				testSuffix + " and test through the exported API",
		})
	}
	return nil, nil
}

func isTestFile(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go")
}
