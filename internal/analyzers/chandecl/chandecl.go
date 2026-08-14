// Package chandecl enforces TS-C12: channel types are declared in one file
// per package.
//
// The rule text scopes itself to "channel types used across goroutines."
// Whether a given channel type demonstrably crosses a goroutine boundary is
// not decidable from one package's AST and types alone — it depends on how
// values of the type are used, possibly from other packages. Wave 1
// resolves that open question by dropping the qualifier: every
// channel-carrying type declaration in a package must colocate, whether or
// not the channel provably crosses goroutines. This is exact and stricter
// than the spec text's literal scope, and it is never silent — colocation
// is checkable in every package that declares more than one such type,
// regardless of how the channel is used.
//
// A type declaration counts if its type expression contains a ChanType
// anywhere within it: a direct alias ("type Events chan Event") and a
// struct with a channel field both count. Anonymous channel types spelled
// out only in a function signature or a var declaration are not type
// declarations and are not counted (known-miss: wave 1 colocates named
// types, not every place a channel type is written).
package chandecl

import (
	"go/ast"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-C12: channel types are declared in one file per package.
var Analyzer = &analysis.Analyzer{
	Name: "chandecl",
	Doc:  "TS-C12: channel types are declared in one file per package",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	byFile := map[string][]*ast.TypeSpec{}
	for _, file := range pass.Files {
		specs := channelDecls(file)
		if len(specs) == 0 {
			continue
		}
		name := filepath.Base(pass.Fset.Position(file.Pos()).Filename)
		byFile[name] = specs
	}
	if len(byFile) < 2 {
		return nil, nil
	}

	files := make([]string, 0, len(byFile))
	for name := range byFile {
		files = append(files, name)
	}
	sort.Strings(files)
	canonical := files[0]

	for _, name := range files[1:] {
		for _, spec := range byFile[name] {
			pass.Report(analysis.Diagnostic{
				Pos:      spec.Pos(),
				Category: "TS-C12",
				Message: "TS-C12: channel type declared outside " + canonical +
					" — move this declaration to " + canonical + " so the communication " +
					"topology and its capacity constants read in one place",
			})
		}
	}
	return nil, nil
}

// channelDecls returns every type declaration in file whose type expression
// contains a channel type: a direct alias or a struct with a channel field.
func channelDecls(file *ast.File) []*ast.TypeSpec {
	found := []*ast.TypeSpec{}
	for _, top := range file.Decls {
		general, ok := top.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if hasChanType(typeSpec.Type) {
				found = append(found, typeSpec)
			}
		}
	}
	return found
}

// hasChanType reports whether expr's type expression contains a ChanType
// anywhere within it.
func hasChanType(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if _, ok := node.(*ast.ChanType); ok {
			found = true
			return false
		}
		return true
	})
	return found
}
