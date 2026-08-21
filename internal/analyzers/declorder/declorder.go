// Package declorder enforces TS-N06 and TS-L05: a single-caller helper
// carries its caller's name, and a struct's file-local declarations read
// type, then constructor, then methods.
//
// TS-N06 only judges an unexported, receiver-less, top-level function whose
// entire call set in the package is certain: every identifier reference to
// it must sit in call position (the Fun of a CallExpr), and the distinct
// FuncDecls doing the calling — excluding the helper calling itself — must
// number exactly one. A helper ever passed as a value has an uncertain call
// set and stays silent; two or more callers stays silent too, because the
// rule only judges a private helper against its one owner. The caller
// itself is exempt when it is main, init, or shaped like a Go test function
// (Test or TestXxx taking a single *testing.T) — none of those names make a
// meaningful prefix. Prefix comparison case-normalizes only the caller
// name's first rune, so an exported ReadSector prefixes readSectorRetry.
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
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-N06 and TS-L05: helper naming and struct declaration
// order.
var Analyzer = &analysis.Analyzer{
	Name: "declorder",
	Doc:  "TS-N06 and TS-L05: helper naming and struct declaration order.",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	checkHelperNaming(pass)
	checkStructOrder(pass)
	return nil, nil
}

// testNamePrefix matches every TestXxx name except the exact name "Test",
// which is checked separately.
var testNamePrefix = regexp.MustCompile(`^Test\p{Lu}`)

// checkHelperNaming enforces TS-N06.
func checkHelperNaming(pass *analysis.Pass) {
	funcs := topLevelFuncs(pass)
	callIdents := callPositionIdents(pass)

	usesByObject := map[types.Object][]*ast.Ident{}
	for ident, obj := range pass.TypesInfo.Uses {
		usesByObject[obj] = append(usesByObject[obj], ident)
	}

	for _, fn := range funcs {
		if fn.Recv != nil || ast.IsExported(fn.Name.Name) {
			continue
		}
		obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
		if !ok {
			continue
		}
		caller, ok := soleCaller(fn, funcs, callIdents, usesByObject[obj])
		if !ok || isExemptCaller(pass, caller) {
			continue
		}
		rename := helperRename{caller: caller.Name.Name, helper: fn.Name.Name}
		if !nameCarriesPrefix(rename) {
			reportHelperNaming(pass, helperCallerDecls{helper: fn, caller: caller})
		}
	}
}

// soleCaller reports the single FuncDecl calling helper by plain
// identifier, or ok=false when the call set is uncertain (a non-call
// reference exists) or the caller count is not exactly one. Self-recursion
// — a call inside helper's own body — never counts as a caller.
func soleCaller(
	helper *ast.FuncDecl,
	funcs []*ast.FuncDecl,
	callIdents map[*ast.Ident]bool,
	uses []*ast.Ident,
) (*ast.FuncDecl, bool) {
	callers := map[*ast.FuncDecl]bool{}
	for _, ident := range uses {
		if !callIdents[ident] {
			return nil, false
		}
		enclosing := enclosingFunc(funcs, ident.Pos())
		if enclosing == nil || enclosing == helper {
			continue
		}
		callers[enclosing] = true
	}
	if len(callers) != 1 {
		return nil, false
	}
	for caller := range callers {
		return caller, true
	}
	return nil, false
}

// isExemptCaller reports whether caller's name never makes a meaningful
// prefix: main, init, or a Go test function.
func isExemptCaller(pass *analysis.Pass, caller *ast.FuncDecl) bool {
	name := caller.Name.Name
	if name == "main" || name == "init" {
		return true
	}
	return isTestShaped(pass, caller)
}

// isTestShaped reports whether fn is shaped like a Go test function: a name
// of Test or TestXxx, taking a single *testing.T parameter.
func isTestShaped(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	if name != "Test" && !testNamePrefix.MatchString(name) {
		return false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	return isTestingT(pass.TypesInfo.TypeOf(fn.Type.Params.List[0].Type))
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

// helperRename pairs a single-caller helper's name with its caller's name —
// the names nameCarriesPrefix checks and suggestName recombines. A named
// struct keeps the two apart, since a swapped caller/helper argument pair is
// exactly the silent bug TS-N07 exists to prevent.
type helperRename struct {
	caller string
	helper string
}

// nameCarriesPrefix reports whether r.helper's name starts with r.caller's
// name, comparing with only the caller name's first rune case-normalized —
// an exported ReadSector prefixes readSectorRetry.
func nameCarriesPrefix(r helperRename) bool {
	if r.caller == "" {
		return false
	}
	return strings.HasPrefix(r.helper, lowerFirst(r.caller))
}

// topLevelFuncs collects every FuncDecl (function or method) declared at
// file scope across the package.
func topLevelFuncs(pass *analysis.Pass) []*ast.FuncDecl {
	funcs := []*ast.FuncDecl{}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				funcs = append(funcs, fn)
			}
		}
	}
	return funcs
}

// callPositionIdents collects every *ast.Ident that is the plain-identifier
// Fun of a CallExpr, across the package.
func callPositionIdents(pass *analysis.Pass) map[*ast.Ident]bool {
	idents := map[*ast.Ident]bool{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ident, ok := call.Fun.(*ast.Ident); ok {
				idents[ident] = true
			}
			return true
		})
	}
	return idents
}

// enclosingFunc returns the top-level FuncDecl among funcs whose body
// contains pos, or nil when pos sits outside every FuncDecl body (for
// example, a package-level var initializer).
func enclosingFunc(funcs []*ast.FuncDecl, pos token.Pos) *ast.FuncDecl {
	for _, fn := range funcs {
		if fn.Body != nil && pos >= fn.Body.Pos() && pos < fn.Body.End() {
			return fn
		}
	}
	return nil
}

// lowerFirst returns s with its first rune lowercased.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// upperFirst returns s with its first rune uppercased.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// suggestName joins r.caller's name (first rune lowercased) with r.helper's
// name (first rune uppercased) into the caller-prefixed rename TS-N06
// suggests.
func suggestName(r helperRename) string {
	return lowerFirst(r.caller) + upperFirst(r.helper)
}

// helperCallerDecls pairs a single-caller helper's FuncDecl with its
// caller's, mirroring helperRename for the AST nodes TS-N06 reports against.
type helperCallerDecls struct {
	helper *ast.FuncDecl
	caller *ast.FuncDecl
}

func reportHelperNaming(pass *analysis.Pass, decls helperCallerDecls) {
	rename := helperRename{caller: decls.caller.Name.Name, helper: decls.helper.Name.Name}
	pass.Report(analysis.Diagnostic{
		Pos:      decls.helper.Pos(),
		Category: "TS-N06",
		Message: "TS-N06: " + decls.helper.Name.Name + " has a single caller (" +
			decls.caller.Name.Name + ") but its name doesn't carry that caller's prefix — " +
			"rename it to " + suggestName(rename) + " so the call site reads as the caller's " +
			"own step",
	})
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
