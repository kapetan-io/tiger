// Package declorder enforces TS-L05: a struct's file-local declarations
// read type, then constructor, then methods.
//
// TS-L05 checks, per struct type declared in a file, that the same file's
// related declarations — a constructor named New<Type>/new<Type>, and every
// method whose receiver base type is the struct — appear after the type,
// and every method appears after the constructor when one exists. It never
// compares across files (a struct's type in one file and a method in
// another is invisible to it) and never inspects nested-type promotion
// inside the struct body (that is TS-K04's job, routed to a later
// analyzer).
package declorder

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-L05: struct declaration order.
var Analyzer = &analysis.Analyzer{
	Name: "declorder",
	Doc:  "TS-L05: struct declaration order.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	checkStructOrder(pass)
	return nil, nil
}

// structInfo pairs a struct type's name with its TypeSpec, scoped to one
// file (TS-L05 never compares across files).
type structInfo struct {
	name string
	spec *ast.TypeSpec
}

// checkStructOrder enforces TS-L05.
func checkStructOrder(pass *analysis.Pass) {
	for _, file := range pass.Files {
		decls := fileDecls(file)
		for _, s := range decls.structs {
			checkOneStructOrder(pass, s, decls.funcs)
		}
	}
}

// fileDeclEntities holds one file's top-level struct type declarations and
// FuncDecls together, since checkStructOrder always needs both at once.
type fileDeclEntities struct {
	structs []structInfo
	funcs   []*ast.FuncDecl
}

// fileDecls partitions file's top-level declarations into its struct type
// declarations and its FuncDecls.
func fileDecls(file *ast.File) fileDeclEntities {
	structs := []structInfo{}
	funcs := []*ast.FuncDecl{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, item := range d.Specs {
				spec, ok := item.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := spec.Type.(*ast.StructType); !ok {
					continue
				}
				structs = append(structs, structInfo{name: spec.Name.Name, spec: spec})
			}
		case *ast.FuncDecl:
			funcs = append(funcs, d)
		}
	}
	return fileDeclEntities{structs: structs, funcs: funcs}
}

// checkOneStructOrder finds s's file-local constructor and methods among
// funcs, then fires TS-L05 at every declaration that precedes something it
// must follow: a constructor before the type, or a method before the type
// or before the constructor.
func checkOneStructOrder(pass *analysis.Pass, s structInfo, funcs []*ast.FuncDecl) {
	var constructor *ast.FuncDecl
	methods := []*ast.FuncDecl{}
	for _, fn := range funcs {
		if fn.Recv == nil {
			if constructor == nil && s.isConstructorName(fn.Name.Name) {
				constructor = fn
			}
			continue
		}
		if name, ok := receiverTypeName(fn); ok && name == s.name {
			methods = append(methods, fn)
		}
	}

	typePos := s.spec.Pos()
	if constructor != nil && constructor.Pos() < typePos {
		reportConstructorBeforeType(pass, constructor, s.name)
	}
	for _, method := range methods {
		switch {
		case method.Pos() < typePos:
			reportMethodBeforeType(pass, method, s.name)
		case constructor != nil && method.Pos() < constructor.Pos():
			reportMethodBeforeConstructor(pass, method, s, constructor)
		}
	}
}

// isConstructorName reports whether name is s's New<Type> or new<Type>
// constructor name.
func (s structInfo) isConstructorName(name string) bool {
	return name == "New"+s.name || name == "new"+s.name
}

// receiverTypeName extracts fn's receiver base type name, unwrapping a
// pointer receiver and a generic receiver's type-parameter brackets.
func receiverTypeName(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return "", false
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, true
	case *ast.IndexExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name, true
		}
	case *ast.IndexListExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name, true
		}
	}
	return "", false
}

func reportConstructorBeforeType(pass *analysis.Pass, constructor *ast.FuncDecl, typeName string) {
	pass.Report(analysis.Diagnostic{
		Pos:      constructor.Pos(),
		Category: "TS-L05",
		Message: "TS-L05: constructor " + constructor.Name.Name + " is declared before its type " +
			typeName + " — move " + typeName + "'s type declaration above its constructor so a " +
			"reader meets the fields before construction",
	})
}

func reportMethodBeforeType(pass *analysis.Pass, method *ast.FuncDecl, typeName string) {
	pass.Report(analysis.Diagnostic{
		Pos:      method.Pos(),
		Category: "TS-L05",
		Message: "TS-L05: method " + method.Name.Name + " is declared before its type " + typeName +
			" — move " + typeName + "'s type declaration above its methods so a reader meets the " +
			"fields first",
	})
}

func reportMethodBeforeConstructor(
	pass *analysis.Pass,
	method *ast.FuncDecl,
	s structInfo,
	constructor *ast.FuncDecl,
) {
	pass.Report(analysis.Diagnostic{
		Pos:      method.Pos(),
		Category: "TS-L05",
		Message: "TS-L05: method " + method.Name.Name + " is declared before its constructor " +
			constructor.Name.Name + " — move " + constructor.Name.Name + " above " + s.name +
			"'s methods so a reader meets construction before behavior",
	})
}
