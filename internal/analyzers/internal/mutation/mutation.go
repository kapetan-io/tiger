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

type Inserts bool

const (
	CountInserts  Inserts = true
	IgnoreInserts Inserts = false
)

type Kind int

const (
	Assign Kind = iota
	Range
	Insert
	Closure
)

type Write struct {
	Node ast.Node
	Kind Kind
}

type Scope struct {
	info   *types.Info
	root   ast.Node
	home   *types.Package
	module string
	sizes  types.Sizes
}

func (s Scope) External(pkg *types.Package) bool {
	if pkg == nil || pkg == s.home {
		return false
	}
	if s.module == "" {
		return true
	}
	return pkg.Path() != s.module && !strings.HasPrefix(pkg.Path(), s.module+"/")
}

func (s Scope) Writes(node ast.Node, v *types.Var) []Write {
	return s.collect(node, func(expr ast.Expr) bool { return s.IsVar(expr, v) }, CountInserts)
}

func (s Scope) FieldWrites(node ast.Node, field *types.Var) []Write {
	return s.collect(node, func(expr ast.Expr) bool { return s.isField(expr, field) }, CountInserts)
}

func (s Scope) Escapes(v *types.Var) bool {
	if v.IsField() {
		return true
	}
	if v.Parent() == v.Pkg().Scope() {
		return true
	}
	return s.addressTaken(func(expr ast.Expr) bool { return s.IsVar(expr, v) }) ||
		s.ClosureWrites(func(expr ast.Expr) bool { return s.IsVar(expr, v) }, v.Pos())
}

func (s Scope) FieldEscapes(field *types.Var) bool {
	return s.addressTaken(func(expr ast.Expr) bool { return s.isField(expr, field) }) ||
		s.ClosureWrites(func(expr ast.Expr) bool { return s.isField(expr, field) }, token.NoPos)
}

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

func isType(t types.Type, want *types.Named) bool {
	named := DerefNamed(t)
	return named != nil && named.Obj() == want.Obj()
}

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

func (s Scope) IsVar(expr ast.Expr, v *types.Var) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && s.info.ObjectOf(ident) == v
}

func (s Scope) isField(expr ast.Expr, field *types.Var) bool {
	selector, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	selection := s.info.Selections[selector]
	return selection != nil && selection.Kind() == types.FieldVal && selection.Obj() == field
}

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

func (s Scope) insertsInto(lhs ast.Expr, match func(ast.Expr) bool) bool {
	index, ok := lhs.(*ast.IndexExpr)
	if !ok || !match(index.X) {
		return false
	}
	_, isMap := s.info.TypeOf(index.X).Underlying().(*types.Map)
	return isMap
}

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

func (s Scope) ClosureWrites(match func(ast.Expr) bool, declared token.Pos) bool {
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

func (s Scope) Sizes() types.Sizes {
	return s.sizes
}

func (s Scope) Root() ast.Node {
	return s.root
}

func (s Scope) Info() *types.Info {
	return s.info
}

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

func (s Scope) defines(write Write) bool {
	if assign, ok := write.Node.(*ast.AssignStmt); ok {
		return assign.Tok == token.DEFINE
	}
	_, isSpec := write.Node.(*ast.ValueSpec)
	return isSpec
}

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

func (s Scope) constant(expr ast.Expr) (*big.Int, bool) {
	value := s.info.Types[expr].Value
	if value == nil || value.Kind() != constant.Int {
		return nil, false
	}
	exact := constant.ToInt(value)
	number, ok := new(big.Int).SetString(exact.ExactString(), 10)
	return number, ok
}

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
