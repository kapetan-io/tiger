// Package testpackage enforces TS-T12: tests live in the external _test package.
package testpackage

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "testpackage",
	Doc:  "TS-T12: tests live in the external _test package.",
	Run:  run,
}

const testSuffix = "_test"

func run(pass *analysis.Pass) (any, error) {
	underTest := map[string]bool{}
	for _, file := range pass.Files {
		if !isTestFile(pass, file) {
			underTest[file.Name.Name] = true
		}
	}
	for _, file := range pass.Files {
		if isTestFile(pass, file) && Internal(file.Name.Name, underTest) {
			pass.Report(analysis.Diagnostic{
				Pos:      file.Name.Pos(),
				Category: "TS-T12",
				Message:  Message(file.Name.Name),
			})
		}
	}
	return nil, nil
}

func Internal(name string, underTest map[string]bool) bool {
	return !strings.HasSuffix(name, testSuffix) || underTest[name]
}

func Message(name string) string {
	return "TS-T12: this test file declares package " + name + ", which gives it " +
		"every unexported name — declare package " + name + testSuffix +
		" and test through the exported API"
}

func isTestFile(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go")
}
