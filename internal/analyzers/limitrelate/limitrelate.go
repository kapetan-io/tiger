// Package limitrelate enforces TS-S21: every limit constant participates in
// a compile-time relational assertion.
//
// Whether a Max or Min constant holds the right number is arithmetic about
// the world and no analyzer can check it. Whether the author related it to
// anything else is checkable: every package-level constant whose name ends
// in Max or Min must appear inside a package-level `const _ = uint(...)`
// expression somewhere in the package, the compile-time relational
// assertion pattern from TS-S17. The analyzer only checks that the name
// appears inside such an expression — it does not judge whether the
// relation is a meaningful one.
package limitrelate

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-S21: every limit constant participates in a
// relational assertion.
var Analyzer = &analysis.Analyzer{
	Name: "limitrelate",
	Doc:  "TS-S21: every limit constant participates in a relational assertion",
	Run:  run,
}

var unsignedConversions = map[string]bool{
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
}

func run(pass *analysis.Pass) (any, error) {
	participating := map[string]bool{}
	var limits []*ast.Ident
	for _, file := range pass.Files {
		for name := range assertionIdentifiers(file) {
			participating[name] = true
		}
		limits = append(limits, limitConstants(file)...)
	}
	reportUnrelated(pass, limits, participating)
	return nil, nil
}

// assertionIdentifiers collects every identifier appearing inside a
// package-level `const _ = ...` expression that contains a conversion to
// an unsigned integer type.
func assertionIdentifiers(file *ast.File) map[string]bool {
	found := map[string]bool{}
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
			collectAssertion(constSpec, found)
		}
	}
	return found
}

func collectAssertion(spec *ast.ValueSpec, found map[string]bool) {
	for i, name := range spec.Names {
		if name.Name != "_" || i >= len(spec.Values) {
			continue
		}
		expr := spec.Values[i]
		if !hasUnsignedConversion(expr) {
			continue
		}
		collectIdentifiers(expr, found)
	}
}

func hasUnsignedConversion(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if ok && unsignedConversions[callee.Name] {
			found = true
		}
		return true
	})
	return found
}

func collectIdentifiers(expr ast.Expr, found map[string]bool) {
	ast.Inspect(expr, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok {
			found[ident.Name] = true
		}
		return true
	})
}

// limitConstants collects every package-level constant name ending in Max
// or Min, declared anywhere in file.
func limitConstants(file *ast.File) []*ast.Ident {
	var limits []*ast.Ident
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
			limits = append(limits, limitNames(constSpec)...)
		}
	}
	return limits
}

func limitNames(spec *ast.ValueSpec) []*ast.Ident {
	var names []*ast.Ident
	for _, name := range spec.Names {
		if strings.HasSuffix(name.Name, "Max") || strings.HasSuffix(name.Name, "Min") {
			names = append(names, name)
		}
	}
	return names
}

func reportUnrelated(pass *analysis.Pass, limits []*ast.Ident, participating map[string]bool) {
	for _, name := range limits {
		if participating[name.Name] {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      name.Pos(),
			Category: "TS-S21",
			Message: "TS-S21: a limit that relates to nothing is a limit nobody reasoned about " +
				"— add const _ = uint(<relation>) relating " + name.Name +
				" to the quantities that bound it",
		})
	}
}
