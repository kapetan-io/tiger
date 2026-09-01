// Package singleimpl enforces TS-X01: no interface with exactly one
// implementation.
//
// This is a whole-program rule (ADR-0010): an interface is declared in one
// package and implemented anywhere in the module, so no single pass can
// count implementations. The per-package pass here exports one fact per
// package — the interfaces and candidate implementer types it declares in
// non-test files — and Finish, the driver-called finish half, resolves
// those names back against the loaded types.Package objects and runs
// types.Implements over the whole set. A type declared in a _test.go file
// never counts as an implementation, and an interface declared in a
// _test.go file is never checked at all — the file suffix is the exact
// boundary state invariant 3 states.
package singleimpl

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer runs the per-package half: it exports Fact and reports nothing
// itself. TS-X01's findings come only from Finish, called once after every
// package has been visited.
var Analyzer = &analysis.Analyzer{
	Name: "singleimpl",
	Doc: "TS-X01: no interface with exactly one implementation; counting runs whole-module " +
		"in the driver's finish step.",
	FactTypes: []analysis.Fact{new(Fact)},
	Run:       run,
}

// Fact is singleimpl's package fact: the names of the interfaces and the
// candidate implementer types this package declares at top level in
// non-test files. Method sets are not serialized — Finish re-resolves each
// name against the loaded types.Package after every package has been
// visited, since one driver.Check call type-checks the whole module as one
// shared universe.
type Fact struct {
	Interfaces []string
	Types      []string
}

// AFact marks Fact as a go/analysis fact.
func (*Fact) AFact() {}

func run(pass *analysis.Pass) (any, error) {
	fact := &Fact{}
	for _, file := range pass.Files {
		if inTestFile(pass, file) {
			continue
		}
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					classify(pass, typeSpec, fact)
				}
			}
		}
	}
	pass.ExportPackageFact(fact)
	return nil, nil
}

// inTestFile reports whether file is a _test.go file, the exact boundary
// TS-X01's test-double exclusion is defined by.
func inTestFile(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go")
}

// classify records spec as an interface or a candidate implementer type,
// skipping generics (no devirtualization story for a type parameter) and
// the empty or no-method interface: nothing can fail to implement a
// contract with no methods, so it is excluded by rule, not by accident.
func classify(pass *analysis.Pass, spec *ast.TypeSpec, fact *Fact) {
	object, ok := pass.TypesInfo.Defs[spec.Name].(*types.TypeName)
	if !ok {
		return
	}
	named, ok := object.Type().(*types.Named)
	if !ok || named.TypeParams().Len() > 0 {
		return
	}
	if iface, ok := named.Underlying().(*types.Interface); ok {
		if iface.NumMethods() > 0 {
			fact.Interfaces = append(fact.Interfaces, spec.Name.Name)
		}
		return
	}
	fact.Types = append(fact.Types, spec.Name.Name)
}
