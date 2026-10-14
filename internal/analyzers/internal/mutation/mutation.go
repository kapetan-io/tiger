// Package mutation finds what can change a variable or struct field inside
// one declaration, for the rules that judge a loop: whether its counter, its
// limit, or its measure moves the way the rule claims. It also knows what a
// variable can hold when every write to it is a constant, and which packages
// sit outside the module under analysis.
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

// Inserts says whether growing a map or channel counts as a write. It does
// when the target is something whose length a loop may be measuring.
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
	// Range is a range clause that reassigns the variable on each pass.
	Range
	// Insert stores into a map or sends on a channel, growing its length.
	Insert
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
	sizes  types.Sizes
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
	return s.collect(node, func(expr ast.Expr) bool { return s.IsVar(expr, v) }, CountInserts)
}

// FieldWrites collects every write to the struct field in node, whatever
// value holds the struct.
func (s Scope) FieldWrites(node ast.Node, field *types.Var) []Write {
	return s.collect(node, func(expr ast.Expr) bool { return s.isField(expr, field) }, CountInserts)
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
	return s.addressTaken(func(expr ast.Expr) bool { return s.IsVar(expr, v) }) || s.ClosureWrites(v)
}

// ClosureWrites reports whether a function literal in scope writes v. A
// literal can run at any point, so its writes cannot be ordered against the
// statements around it.
func (s Scope) ClosureWrites(v *types.Var) bool {
	return s.closureWrites(func(expr ast.Expr) bool { return s.IsVar(expr, v) }, v.Pos())
}

// ClosureWritesField reports whether a function literal in scope writes the
// struct field, whatever value holds the struct.
func (s Scope) ClosureWritesField(field *types.Var) bool {
	return s.closureWrites(func(expr ast.Expr) bool { return s.isField(expr, field) }, token.NoPos)
}

// FieldEscapes reports whether something outside the loop body could write
// the field: its address is taken or a function literal in scope writes it.
func (s Scope) FieldEscapes(field *types.Var) bool {
	return s.addressTaken(func(expr ast.Expr) bool { return s.isField(expr, field) }) ||
		s.ClosureWritesField(field)
}

// ReceivesOwner reports whether a call in node receives a value of the
// struct type that owns a field, or a pointer to one. The call may write the
// field through it, so the field cannot be trusted across the call. A struct
// declared outside the module is the exception: its own code does not reach
// into the fields a caller reads, such as testing.B's N.
func (s Scope) ReceivesOwner(node ast.Node, owner types.Type) bool {
	want := DerefNamed(owner)
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
	named := DerefNamed(t)
	return named != nil && named.Obj() == want.Obj()
}

// DerefNamed returns the named type behind t or a pointer to t.
func DerefNamed(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return nil
	}
	return named
}

// IsVar reports whether expr is a use of v.
func (s Scope) IsVar(expr ast.Expr, v *types.Var) bool {
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
				found = append(found, Write{Node: stmt, Kind: Insert})
			}
		}
	case *ast.ValueSpec:
		for _, name := range stmt.Names {
			if match(name) {
				found = append(found, Write{Node: stmt, Kind: Assign})
			}
		}
	case *ast.SendStmt:
		if inserts == CountInserts && match(stmt.Chan) {
			found = append(found, Write{Node: stmt, Kind: Insert})
		}
	case *ast.IncDecStmt:
		if match(stmt.X) {
			found = append(found, Write{Node: stmt, Kind: Assign})
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
	_, isMap := s.info.TypeOf(index.X).Underlying().(*types.Map)
	return isMap
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
		ownsTarget := declared != token.NoPos &&
			literal.Pos() <= declared && declared < literal.End()
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
	sizes := pass.TypesSizes
	if sizes == nil {
		sizes = types.SizesFor("gc", "amd64")
	}
	var scopes []Scope
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			scopes = append(scopes, Scope{
				info: pass.TypesInfo, root: decl, home: pass.Pkg, module: module, sizes: sizes,
			})
		}
	}
	return scopes
}

// Sizes returns the target's type sizes, the common 64-bit layout when the
// driver supplied none.
func (s Scope) Sizes() types.Sizes {
	return s.sizes
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
	defined := false
	for _, write := range s.Writes(s.root, v) {
		defined = defined || s.defines(write)
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
	return span, defined && span.Low != nil
}

// defines reports whether a write declares its variable, as opposed to
// assigning one declared elsewhere, such as a parameter that arrives with a
// value of its own.
func (s Scope) defines(write Write) bool {
	if assign, ok := write.Node.(*ast.AssignStmt); ok {
		return assign.Tok == token.DEFINE
	}
	_, isSpec := write.Node.(*ast.ValueSpec)
	return isSpec
}

// assigned returns the constant a write stores into v.
func (s Scope) assigned(write Write, v *types.Var) (*big.Int, bool) {
	match := func(expr ast.Expr) bool { return s.IsVar(expr, v) }
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
		if len(stmt.Values) == 0 {
			return big.NewInt(0), true
		}
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

// Mentions reports whether expr reads v anywhere inside it.
func (s Scope) Mentions(expr ast.Expr, v *types.Var) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if ok && s.info.ObjectOf(ident) == v {
			found = true
		}
		return !found
	})
	return found
}

// PassedToCall reports whether a call in node takes v as an argument or as the
// receiver of a method. A map handed to a function can be inserted into there,
// unlike a slice, whose length the callee cannot change.
func (s Scope) PassedToCall(node ast.Node, v *types.Var) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		if s.info.Types[call.Fun].IsType() {
			return !found
		}
		if ident, isIdent := ast.Unparen(call.Fun).(*ast.Ident); isIdent {
			if _, builtin := s.info.Uses[ident].(*types.Builtin); builtin {
				return !found
			}
		}
		for _, arg := range call.Args {
			if s.IsVar(arg, v) {
				found = true
			}
		}
		if selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr); isSelector {
			if s.IsVar(selector.X, v) && s.info.Selections[selector] != nil {
				found = true
			}
		}
		return !found
	})
	return found
}

// Parameters collects every function parameter declared in the scope's
// declaration, the declaration's own and those of function literals inside it.
func (s Scope) Parameters() map[*types.Var]bool {
	found := map[*types.Var]bool{}
	ast.Inspect(s.root, func(node ast.Node) bool {
		var signature *ast.FuncType
		switch fn := node.(type) {
		case *ast.FuncDecl:
			signature = fn.Type
		case *ast.FuncLit:
			signature = fn.Type
		default:
			return true
		}
		for _, field := range signature.Params.List {
			for _, name := range field.Names {
				if v, ok := s.info.Defs[name].(*types.Var); ok {
					found[v] = true
				}
			}
		}
		return true
	})
	return found
}
