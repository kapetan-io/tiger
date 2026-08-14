// Package tablename enforces TS-T10: table-driven tests have named cases.
//
// A slice-of-anonymous-struct table inside a Test/Benchmark/Fuzz function
// explains a failing case by its field values alone unless one field is
// named "name" or "Name" — the value t.Run and a failure trace both print.
// A map[string]struct{...} table is named by construction and stays silent.
// A slice of a declared struct type is not examined; see the ts-t10 corpus
// known-miss case for that gap.
package tablename

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-T10: table-driven tests have named cases.
var Analyzer = &analysis.Analyzer{
	Name: "tablename",
	Doc:  "TS-T10: table-driven tests have named cases.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if !strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !isTestFuncName(fn.Name.Name) {
				continue
			}
			checkFunc(pass, fn)
		}
	}
	return nil, nil
}

// isTestFuncName reports whether name matches the Test/Benchmark/Fuzz
// prefixes the rule scopes to.
func isTestFuncName(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// checkFunc walks a Test/Benchmark/Fuzz function body for unnamed
// table-driven case structs.
func checkFunc(pass *analysis.Pass, fn *ast.FuncDecl) {
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		composite, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		checkComposite(pass, composite)
		return true
	})
}

// checkComposite reports a slice-of-anonymous-struct literal that declares
// no "name"/"Name" field.
func checkComposite(pass *analysis.Pass, composite *ast.CompositeLit) {
	array, ok := composite.Type.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return
	}
	spec, ok := array.Elt.(*ast.StructType)
	if !ok || hasNameField(spec) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      composite.Pos(),
		Category: "TS-T10",
		Message: "TS-T10: table-driven test case struct has no name field — add a `name string` " +
			"(or `Name string`) field so a failing case explains itself in the test output",
	})
}

// hasNameField reports whether spec declares a field literally named "name"
// or "Name".
func hasNameField(spec *ast.StructType) bool {
	for _, field := range spec.Fields.List {
		for _, name := range field.Names {
			if name.Name == "name" || name.Name == "Name" {
				return true
			}
		}
	}
	return false
}
