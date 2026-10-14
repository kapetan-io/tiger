// Package ownership answers whether code a package runs can reach a
// close, a send, or a Wait on a channel or group a function uses.
//
// TS-S03, TS-C05 and TS-C02 all ask the same question of different calls:
// is there code in this package that closes this channel, or waits on this
// group, and can the package's exported surface get there? The answer
// depends on where the object lives. A local is the function's own, so only
// that function can act on it. A package variable can be acted on by
// anything an exported function or method reaches. A struct field belongs to
// its type, so only a method of that type that an exported method of that
// type reaches counts. A parameter belongs to the caller, so the answer
// comes from the arguments the package passes in.
//
// The reach is a static walk over the package's own declarations. A call
// through an interface or a function value is not followed, and code that is
// only called from dead code still counts as reached when something
// reachable names it.
package ownership

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/worklist"
)

// Argument is an expression a call passes for a parameter, with the
// declaration holding the call.
type Argument struct {
	Expr ast.Expr
	Decl *ast.FuncDecl
}

// Index is a package's declarations, the references between them, and the
// arguments its calls pass.
type Index struct {
	info    *types.Info
	fset    *token.FileSet
	pkg     *types.Package
	decls   []*ast.FuncDecl
	byFunc  map[*types.Func]*ast.FuncDecl
	edges   map[*ast.FuncDecl][]*ast.FuncDecl
	args    map[*types.Var][]Argument
	params  map[*types.Var]bool
	reached map[*types.TypeName]map[*ast.FuncDecl]bool
}

// New indexes the declarations of the package under analysis.
func New(pass *analysis.Pass) *Index {
	index := &Index{
		info:    pass.TypesInfo,
		fset:    pass.Fset,
		pkg:     pass.Pkg,
		byFunc:  map[*types.Func]*ast.FuncDecl{},
		edges:   map[*ast.FuncDecl][]*ast.FuncDecl{},
		args:    map[*types.Var][]Argument{},
		params:  map[*types.Var]bool{},
		reached: map[*types.TypeName]map[*ast.FuncDecl]bool{},
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			index.decls = append(index.decls, fn)
			if obj, isFunc := index.info.Defs[fn.Name].(*types.Func); isFunc {
				index.byFunc[obj] = fn
			}
		}
	}
	for _, fn := range index.decls {
		index.edges[fn] = index.references(fn)
		index.collectArguments(fn)
	}
	for _, file := range pass.Files {
		index.collectParameters(file)
	}
	return index
}

// collectParameters records every parameter and receiver in file, function
// literals included.
func (index *Index) collectParameters(file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		var lists []*ast.FieldList
		switch fn := node.(type) {
		case *ast.FuncDecl:
			lists = []*ast.FieldList{fn.Recv, fn.Type.Params}
		case *ast.FuncLit:
			lists = []*ast.FieldList{fn.Type.Params}
		}
		for _, list := range lists {
			if list == nil {
				continue
			}
			for _, field := range list.List {
				for _, name := range field.Names {
					if param, ok := index.info.Defs[name].(*types.Var); ok {
						index.params[param] = true
					}
				}
			}
		}
		return true
	})
}

// IsParameter reports whether v is a parameter or receiver of a function in
// the package.
func (index *Index) IsParameter(v *types.Var) bool {
	return index.params[v]
}

// DeclOf returns the declaration holding node, or nil for code outside any
// function, such as a package variable's initializer.
func (index *Index) DeclOf(node ast.Node) *ast.FuncDecl {
	for _, fn := range index.decls {
		if fn.Pos() <= node.Pos() && node.End() <= fn.End() {
			return fn
		}
	}
	return nil
}

// Decls returns the package's function declarations that have a body, in
// source order.
func (index *Index) Decls() []*ast.FuncDecl {
	return index.decls
}

// DeclFor returns the declaration of a package function or method, or nil for
// one declared elsewhere.
func (index *Index) DeclFor(fn *types.Func) *ast.FuncDecl {
	return index.byFunc[fn.Origin()]
}

// Object returns the variable an expression names: a plain identifier, or the
// field or package variable a selector picks. It returns nil for anything
// else, such as a call result or an element of a slice.
func (index *Index) Object(expr ast.Expr) *types.Var {
	expr = ast.Unparen(expr)
	var ident *ast.Ident
	switch shape := expr.(type) {
	case *ast.Ident:
		ident = shape
	case *ast.SelectorExpr:
		ident = shape.Sel
	default:
		return nil
	}
	obj, ok := index.info.ObjectOf(ident).(*types.Var)
	if !ok {
		return nil
	}
	return obj.Origin()
}

// Arguments returns what the package's calls pass for a parameter.
func (index *Index) Arguments(param *types.Var) []Argument {
	return index.args[param]
}

// Link pairs the declaration that acts on an object, by closing it,
// sending on it, or waiting on it, with the declaration that depends on the
// action. Either is nil for code outside any function.
type Link struct {
	Actor  *ast.FuncDecl
	Reader *ast.FuncDecl
}

// Reaches reports whether the actor can act on obj for the reader. A local
// belongs to the one function that declares it, or to the one initializer
// outside any function (both nil). A field also counts when the actor and the
// reader are the same function. An actor outside any function never reaches a
// field or a package variable.
func (index *Index) Reaches(obj *types.Var, link Link) bool {
	switch {
	case obj.IsField():
		if link.Actor == nil {
			return false
		}
		return link.Actor == link.Reader || index.fieldReached(obj, link.Actor)
	case obj.Parent() == index.pkg.Scope():
		return link.Actor != nil && index.reachedFrom(nil)[link.Actor]
	default:
		return link.Actor == link.Reader
	}
}

// fieldReached reports whether site is a method of the type that owns field,
// reached from an exported method of that type.
func (index *Index) fieldReached(field *types.Var, site *ast.FuncDecl) bool {
	owner := index.receiverType(site)
	if owner == nil {
		return false
	}
	found, _, _ := types.LookupFieldOrMethod(owner.Type(), true, index.pkg, field.Name())
	member, ok := found.(*types.Var)
	if !ok || member.Origin() != field {
		return false
	}
	return index.reachedFrom(owner)[site]
}

// receiverType returns the named type fn is a method of, or nil.
func (index *Index) receiverType(fn *ast.FuncDecl) *types.TypeName {
	if fn.Recv == nil {
		return nil
	}
	obj, ok := index.info.Defs[fn.Name].(*types.Func)
	if !ok {
		return nil
	}
	recv := obj.Signature().Recv()
	if recv == nil {
		return nil
	}
	typ := recv.Type()
	if ptr, isPtr := types.Unalias(typ).(*types.Pointer); isPtr {
		typ = ptr.Elem()
	}
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok {
		return nil
	}
	return named.Origin().Obj()
}

// reachedFrom returns every declaration reachable from the exported methods
// of owner, or from every exported function and method when owner is nil.
// The entry points main and init count as exported.
func (index *Index) reachedFrom(owner *types.TypeName) map[*ast.FuncDecl]bool {
	if seen, ok := index.reached[owner]; ok {
		return seen
	}
	seen := map[*ast.FuncDecl]bool{}
	work := []*ast.FuncDecl{}
	for _, fn := range index.decls {
		if index.isRoot(fn, owner) {
			seen[fn] = true
			work = append(work, fn)
		}
	}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		for _, next := range index.edges[work[i]] {
			if !seen[next] {
				seen[next] = true
				work = append(work, next)
			}
		}
	}
	assert.Ok(i == len(work), "reachedFrom: worklist reached worklist.NodesMax")
	index.reached[owner] = seen
	return seen
}

// isRoot reports whether fn starts a walk for owner. Code in a _test.go file
// never does: a close that only a test performs is not the package's.
func (index *Index) isRoot(fn *ast.FuncDecl, owner *types.TypeName) bool {
	if strings.HasSuffix(index.fset.PositionFor(fn.Pos(), false).Filename, "_test.go") {
		return false
	}
	if owner != nil {
		return fn.Name.IsExported() && index.receiverType(fn) == owner
	}
	if fn.Recv == nil {
		return fn.Name.IsExported() || fn.Name.Name == "main" || fn.Name.Name == "init"
	}
	return fn.Name.IsExported()
}

// references lists the package declarations fn names anywhere in its body,
// as calls or as function values, so a method handed to wg.Go counts.
func (index *Index) references(fn *ast.FuncDecl) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		callee, isFunc := index.info.Uses[ident].(*types.Func)
		if !isFunc {
			return true
		}
		if target, known := index.byFunc[callee.Origin()]; known {
			out = append(out, target)
		}
		return true
	})
	return out
}

// collectArguments records the arguments fn's calls pass to package
// functions and to function literals called on the spot.
func (index *Index) collectArguments(fn *ast.FuncDecl) {
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		params := index.parameters(call)
		for i := 0; i < len(params) && i < len(call.Args); i++ {
			passed := Argument{Expr: call.Args[i], Decl: fn}
			index.args[params[i]] = append(index.args[params[i]], passed)
		}
		return true
	})
}

// parameters returns the parameter variables of the function a call invokes
// when it is a package function or a function literal, in order.
func (index *Index) parameters(call *ast.CallExpr) []*types.Var {
	var list *ast.FieldList
	switch callee := ast.Unparen(call.Fun).(type) {
	case *ast.FuncLit:
		list = callee.Type.Params
	case *ast.Ident:
		list = index.declaredParams(index.info.Uses[callee])
	case *ast.SelectorExpr:
		list = index.declaredParams(index.info.Uses[callee.Sel])
	}
	if list == nil {
		return nil
	}
	var params []*types.Var
	for _, field := range list.List {
		for _, name := range field.Names {
			if param, ok := index.info.Defs[name].(*types.Var); ok {
				params = append(params, param)
			}
		}
	}
	return params
}

// declaredParams returns the parameter list of a package function, or nil.
func (index *Index) declaredParams(obj types.Object) *ast.FieldList {
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	decl, known := index.byFunc[fn.Origin()]
	if !known {
		return nil
	}
	return decl.Type.Params
}

// Definitions returns the expressions assigned to local in decl, one per
// assignment or declaration that gives it a value. A value that comes from a
// multi-value call is the call itself, whichever result local takes.
func (index *Index) Definitions(local *types.Var, decl *ast.FuncDecl) []ast.Expr {
	var out []ast.Expr
	if decl == nil {
		return out
	}
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			if stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE {
				assigned := binding{targets: stmt.Lhs, values: stmt.Rhs}
				out = append(out, index.matching(local, assigned)...)
			}
		case *ast.ValueSpec:
			names := make([]ast.Expr, len(stmt.Names))
			for i, name := range stmt.Names {
				names[i] = name
			}
			declared := binding{targets: names, values: stmt.Values}
			out = append(out, index.matching(local, declared)...)
		}
		return true
	})
	return out
}

// binding is the two sides of an assignment or declaration.
type binding struct {
	targets []ast.Expr
	values  []ast.Expr
}

// matching pairs each target naming local with its value.
func (index *Index) matching(local *types.Var, assigned binding) []ast.Expr {
	var out []ast.Expr
	for i, target := range assigned.targets {
		if index.Object(target) != local {
			continue
		}
		switch {
		case len(assigned.targets) == len(assigned.values):
			out = append(out, assigned.values[i])
		case len(assigned.values) == 1:
			out = append(out, assigned.values[0])
		}
	}
	return out
}
