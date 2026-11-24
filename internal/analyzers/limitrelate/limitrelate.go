// Package limitrelate enforces TS-S21: every limit constant participates in
// a compile-time relational assertion.
//
// Whether a Max or Min constant holds the right number is arithmetic about
// the world and no analyzer can check it. Whether the author related it to
// anything else is checkable: every package-level constant whose name ends
// in Max or Min must appear inside a package-level `const _ = uint(...)`
// expression somewhere in the package, the compile-time relational
// assertion pattern from TS-S17. An unexported limit must also be used
// somewhere outside such expressions, or it bounds nothing the program
// does. The analyzer matches names and does not judge whether a relation
// is a meaningful one.
package limitrelate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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
	used := map[string]bool{}
	var limits []*ast.Ident
	for _, file := range pass.Files {
		for name := range assertionIdentifiers(file) {
			participating[name] = true
		}
		for name := range usesOutsideAssertions(file) {
			used[name] = true
		}
		limits = append(limits, limitConstants(file)...)
	}
	for name := range usesInExcludedFiles(pass) {
		used[name] = true
	}
	reportUnrelated(pass, limits, participating)
	guardOnly := map[string]bool{}
	for name := range participating {
		if !used[name] {
			guardOnly[name] = true
		}
	}
	reportGuardOnly(pass, limits, guardOnly)
	return nil, nil
}

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

func usesInExcludedFiles(pass *analysis.Pass) map[string]bool {
	found := map[string]bool{}
	if len(pass.Files) == 0 {
		return found
	}
	loaded := map[string]bool{}
	var dirs []string
	for _, file := range pass.Files {
		name := pass.Fset.File(file.Pos()).Name()
		loaded[name] = true
		dirs = append(dirs, filepath.Dir(name))
	}
	slices.Sort(dirs)
	for _, dir := range slices.Compact(dirs) {
		for name := range unloadedUses(pass, dir, loaded) {
			found[name] = true
		}
	}
	return found
}

func unloadedUses(pass *analysis.Pass, dir string, loaded map[string]bool) map[string]bool {
	found := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || loaded[path] {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil || file.Name.Name != pass.Pkg.Name() {
			continue
		}
		for name := range usesOutsideAssertions(file) {
			found[name] = true
		}
	}
	return found
}

func usesOutsideAssertions(file *ast.File) map[string]bool {
	skip := map[ast.Node]bool{}
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
			for _, name := range constSpec.Names {
				skip[name] = true
			}
			for i, name := range constSpec.Names {
				if name.Name != "_" || i >= len(constSpec.Values) {
					continue
				}
				if hasUnsignedConversion(constSpec.Values[i]) {
					skip[constSpec.Values[i]] = true
				}
			}
		}
	}
	skipBlankBindings(file, skip)
	found := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil || skip[node] {
			return false
		}
		if ident, ok := node.(*ast.Ident); ok {
			found[ident.Name] = true
		}
		return true
	})
	return found
}

func skipBlankBindings(file *ast.File, skip map[ast.Node]bool) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch bind := node.(type) {
		case *ast.ValueSpec:
			if allBlank(bind.Names) && allIdents(bind.Values) {
				for _, value := range bind.Values {
					skip[value] = true
				}
			}
		case *ast.AssignStmt:
			if bind.Tok == token.ASSIGN && allIdents(bind.Rhs) &&
				blankLhs(bind.Lhs) {
				for _, value := range bind.Rhs {
					skip[value] = true
				}
			}
		}
		return true
	})
}

func allBlank(names []*ast.Ident) bool {
	for _, name := range names {
		if name.Name != "_" {
			return false
		}
	}
	return true
}

func blankLhs(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if ident, ok := expr.(*ast.Ident); !ok || ident.Name != "_" {
			return false
		}
	}
	return true
}

func allIdents(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if _, ok := expr.(*ast.Ident); !ok {
			return false
		}
	}
	return len(exprs) > 0
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
			Message: "TS-S21: " + name.Name + " is a limit that no assertion relates to any " +
				"other quantity — add a compile-time relation such as " +
				"const _ = uint(bufferSize - " + name.Name + "), or if it depends on nothing, " +
				"const _ = uint(" + name.Name + " - 1)",
		})
	}
}

func reportGuardOnly(pass *analysis.Pass, limits []*ast.Ident, guardOnly map[string]bool) {
	for _, name := range limits {
		if !guardOnly[name.Name] || name.IsExported() {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:      name.Pos(),
			Category: "TS-S21",
			Message: "TS-S21: " + name.Name + " is a limit that only appears inside a relational " +
				"assertion, so it bounds nothing — use it where the limit applies, or delete it",
		})
	}
}
