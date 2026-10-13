// Package mutation finds what can change a variable or struct field while a
// loop runs, for the rules that judge whether a loop measure moves the way
// the rule claims.
//
// Variables are matched by object identity, never by name, so a shadowing
// copy is a different variable. Writes inside the loop come from Writes and
// FieldWrites. Writes the loop body cannot see (a pointer taken before the
// loop, a closure made before the loop, a call that receives the owning
// struct) come from Scope.Escapes, Scope.FieldEscapes and
// ReceivesOwner; any of them defeats a proof.
package mutation

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/intrange"
)

// Inserts says whether a map insert counts as a write. It does when the
// target is a whole variable whose length a loop may be measuring.
type Inserts bool

const (
	// CountInserts counts m[k] = v as a write to the map m.
	CountInserts Inserts = true
	// IgnoreInserts does not.
	IgnoreInserts Inserts = false
)

// Kind says how a statement changes a variable.
type Kind int

const (
	// Assign is a plain, compound, or define assignment to the variable.
	Assign Kind = iota
	// IncDec is a ++ or -- statement.
	IncDec
	// Range is a range clause that reassigns the variable on each pass.
	Range
	// MapInsert stores into a map variable, growing its length.
	MapInsert
	// Closure is a write inside a function literal.
	Closure
)

// Write is one statement that changes a variable or field.
type Write struct {
	Node ast.Node
	Kind Kind
}

// Scope is the declaration a loop sits in, the widest place a pointer or
// closure that outlives the loop body could be made.
type Scope struct {
	info   *types.Info
	root   ast.Node
	home   *types.Package
	module string
}

// External reports whether pkg is declared outside the module under
// analysis. Without module information, any package but the one under
// analysis counts as outside.
func (s Scope) External(pkg *types.Package) bool {
	if pkg == nil || pkg == s.home {
		return false
	}
	if s.module == "" {
		return true
	}
	return pkg.Path() != s.module && !strings.HasPrefix(pkg.Path(), s.module+"/")
}

// Writes collects every write to v in node. A write inside a function
// literal is reported as Closure, because the literal may run any number of
// times.
func (s Scope) Writes(node ast.Node, v *types.Var) []Write {
	return s.collect(node, func(expr ast.Expr) bool { return s.isVar(expr, v) }, CountInserts)
}

// FieldWrites collects every write to the struct field in node, whatever
// value holds the struct.
func (s Scope) FieldWrites(node ast.Node, field *types.Var) []Write {
	return s.collect(node, func(expr ast.Expr) bool { return s.isField(expr, field) }, IgnoreInserts)
}

// Escapes reports whether something outside the loop body could write v: it
// lives outside any function, its address is taken, a pointer-receiver
// method is called on it, or a function literal in scope writes it.
func (s Scope) Escapes(v *types.Var) bool {
	if v.IsField() {
		return true
	}
	if v.Parent() == v.Pkg().Scope() {
		return true
	}
	return s.addressTaken(func(expr ast.Expr) bool { return s.isVar(expr, v) }) ||
		s.closureWrites(func(expr ast.Expr) bool { return s.isVar(expr, v) }, v.Pos())
}

// FieldEscapes reports whether something outside the loop body could write
// the field: its address is taken or a function literal in scope writes it.
func (s Scope) FieldEscapes(field *types.Var) bool {
	return s.addressTaken(func(expr ast.Expr) bool { return s.isField(expr, field) }) ||
		s.closureWrites(func(expr ast.Expr) bool { return s.isField(expr, field) }, token.NoPos)
}

// ReceivesOwner reports whether a call in node receives a value of the
// struct type that owns a field, or a pointer to one. The call may write the
// field through it, so the field cannot be trusted across the call. A struct
// declared outside the module is the exception: its own code does not reach
// into the fields a caller reads, such as testing.B's N.
func (s Scope) ReceivesOwner(node ast.Node, owner types.Type) bool {
	want := derefNamed(owner)
	if want == nil || s.External(want.Obj().Pkg()) {
		return false
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && s.receives(call, want) {
			found = true
		}
		return !found
	})
	return found
}

// receives reports whether call passes want, or a pointer to it, as an
// argument or as the receiver of a method.
func (s Scope) receives(call *ast.CallExpr, want *types.Named) bool {
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		if isType(s.info.TypeOf(selector.X), want) {
			return true
		}
	}
	for _, arg := range call.Args {
		if isType(s.info.TypeOf(arg), want) {
			return true
		}
	}
	return false
}

// isType reports whether t is want or a pointer to want.
func isType(t types.Type, want *types.Named) bool {
	named := derefNamed(t)
	return named != nil && named.Obj() == want.Obj()
}

// derefNamed returns the named type behind t or a pointer to t.
func derefNamed(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, _ := t.(*types.Named)
	return named
}

// isVar reports whether expr is a use of v.
func (s Scope) isVar(expr ast.Expr, v *types.Var) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && s.info.ObjectOf(ident) == v
}

// isField reports whether expr selects the field.
func (s Scope) isField(expr ast.Expr, field *types.Var) bool {
	selector, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	selection := s.info.Selections[selector]
	return selection != nil && selection.Kind() == types.FieldVal && selection.Obj() == field
}

// collect walks node for statements whose target satisfies match. A write
// inside a function literal is reported as Closure.
func (s Scope) collect(node ast.Node, match func(ast.Expr) bool, inserts Inserts) []Write {
	var found []Write
	var literals []*ast.FuncLit
	ast.Inspect(node, func(n ast.Node) bool {
		if literal, ok := n.(*ast.FuncLit); ok {
			literals = append(literals, literal)
		}
		found = append(found, s.targets(n, match, inserts)...)
		return true
	})
	for i, write := range found {
		for _, literal := range literals {
			if literal.Pos() <= write.Node.Pos() && write.Node.End() <= literal.End() {
				found[i].Kind = Closure
			}
		}
	}
	return found
}

// targets returns the writes one node makes to expressions satisfying match.
func (s Scope) targets(n ast.Node, match func(ast.Expr) bool, inserts Inserts) []Write {
	var found []Write
	switch stmt := n.(type) {
	case *ast.AssignStmt:
		for _, lhs := range stmt.Lhs {
			if match(lhs) {
				found = append(found, Write{Node: stmt, Kind: Assign})
			}
			if inserts == CountInserts && s.insertsInto(lhs, match) {
				found = append(found, Write{Node: stmt, Kind: MapInsert})
			}
		}
	case *ast.ValueSpec:
		for _, name := range stmt.Names {
			if match(name) {
				found = append(found, Write{Node: stmt, Kind: Assign})
			}
		}
	case *ast.IncDecStmt:
		if match(stmt.X) {
			found = append(found, Write{Node: stmt, Kind: IncDec})
		}
	case *ast.RangeStmt:
		if stmt.Key != nil && match(stmt.Key) {
			found = append(found, Write{Node: stmt, Kind: Range})
		}
		if stmt.Value != nil && match(stmt.Value) {
			found = append(found, Write{Node: stmt, Kind: Range})
		}
	}
	return found
}

// insertsInto reports whether lhs is m[k] with m satisfying match and m a
// map.
func (s Scope) insertsInto(lhs ast.Expr, match func(ast.Expr) bool) bool {
	index, ok := lhs.(*ast.IndexExpr)
	if !ok || !match(index.X) {
		return false
	}
	mapType, _ := s.info.TypeOf(index.X).Underlying().(*types.Map)
	return mapType != nil
}

// addressTaken reports whether &target appears anywhere in scope, or a
// pointer-receiver method is called on a value satisfying match.
func (s Scope) addressTaken(match func(ast.Expr) bool) bool {
	found := false
	ast.Inspect(s.root, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.UnaryExpr:
			if expr.Op == token.AND && match(expr.X) {
				found = true
			}
		case *ast.SelectorExpr:
			if match(expr.X) && s.pointerMethod(expr) {
				found = true
			}
		}
		return !found
	})
	return found
}

// pointerMethod reports whether selector names a method with a pointer
// receiver called on an addressable value, which takes its address.
func (s Scope) pointerMethod(selector *ast.SelectorExpr) bool {
	selection := s.info.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return false
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	signature, ok := method.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}
	_, pointer := signature.Recv().Type().(*types.Pointer)
	if !pointer {
		return false
	}
	_, held := s.info.TypeOf(selector.X).(*types.Pointer)
	return !held
}

// closureWrites reports whether any function literal in scope writes a
// target satisfying match. A literal that declares the target holds the
// target's own writes, so it counts only for a target declared outside it;
// token.NoPos says the target is declared nowhere in scope, as a field is.
func (s Scope) closureWrites(match func(ast.Expr) bool, declared token.Pos) bool {
	found := false
	ast.Inspect(s.root, func(n ast.Node) bool {
		literal, ok := n.(*ast.FuncLit)
		if !ok {
			return !found
		}
		ownsTarget := declared != token.NoPos && literal.Pos() <= declared && declared < literal.End()
		if !ownsTarget && len(s.collect(literal.Body, match, CountInserts)) > 0 {
			found = true
		}
		return !found
	})
	return found
}

// Declarations returns one scope per top-level declaration in the files
// under analysis, the unit a loop's escapes are judged in.
func Declarations(pass *analysis.Pass) []Scope {
	module := ""
	if pass.Module != nil {
		module = pass.Module.Path
	}
	var scopes []Scope
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			scopes = append(scopes, Scope{
				info: pass.TypesInfo, root: decl, home: pass.Pkg, module: module,
			})
		}
	}
	return scopes
}

// Root returns the declaration the scope covers.
func (s Scope) Root() ast.Node {
	return s.root
}

// Info returns the type information the scope resolves identifiers with.
func (s Scope) Info() *types.Info {
	return s.info
}

// ConstantRange returns the smallest and largest value expr can take when it
// is a constant, or a local variable whose every assignment is a constant.
// It reports false for anything else, including a variable something
// outside its writes could change.
func (s Scope) ConstantRange(expr ast.Expr) (intrange.Span, bool) {
	if value, constant := s.constant(expr); constant {
		return intrange.Span{Low: value, High: value}, true
	}
	ident, isIdent := ast.Unparen(expr).(*ast.Ident)
	if !isIdent {
		return intrange.Span{}, false
	}
	v, isVar := s.info.ObjectOf(ident).(*types.Var)
	if !isVar || s.Escapes(v) {
		return intrange.Span{}, false
	}
	var span intrange.Span
	for _, write := range s.Writes(s.root, v) {
		value, known := s.assigned(write, v)
		if !known {
			return intrange.Span{}, false
		}
		if span.Low == nil || value.Cmp(span.Low) < 0 {
			span.Low = value
		}
		if span.High == nil || value.Cmp(span.High) > 0 {
			span.High = value
		}
	}
	return span, span.Low != nil
}

// assigned returns the constant a write stores into v.
func (s Scope) assigned(write Write, v *types.Var) (*big.Int, bool) {
	match := func(expr ast.Expr) bool { return s.isVar(expr, v) }
	switch stmt := write.Node.(type) {
	case *ast.AssignStmt:
		if stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE {
			return nil, false
		}
		for i, lhs := range stmt.Lhs {
			if match(lhs) && len(stmt.Lhs) == len(stmt.Rhs) {
				return s.constant(stmt.Rhs[i])
			}
		}
	case *ast.ValueSpec:
		for i, name := range stmt.Names {
			if match(name) && len(stmt.Names) == len(stmt.Values) {
				return s.constant(stmt.Values[i])
			}
		}
	}
	return nil, false
}

// constant returns expr's value when the compiler folded it to an integer.
func (s Scope) constant(expr ast.Expr) (*big.Int, bool) {
	value := s.info.Types[expr].Value
	if value == nil || value.Kind() != constant.Int {
		return nil, false
	}
	exact := constant.ToInt(value)
	number, ok := new(big.Int).SetString(exact.ExactString(), 10)
	return number, ok
}
