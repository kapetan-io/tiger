package stopcase

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/analyzers/internal/ownership"
	"github.com/kapetan-io/tiger/internal/worklist"
)

var derivers = map[string]bool{
	"WithCancel": true, "WithCancelCause": true, "WithTimeout": true, "WithTimeoutCause": true,
	"WithDeadline": true, "WithDeadlineCause": true, "WithValue": true,
}

type producer struct {
	node ast.Node
	decl *ast.FuncDecl
}

type Finder struct {
	info   *types.Info
	index  *ownership.Index
	closed map[*types.Var][]producer
	sent   map[*types.Var][]producer
	params []*types.Var
}

func New(pass *analysis.Pass) *Finder {
	finder := &Finder{
		info:   pass.TypesInfo,
		index:  ownership.New(pass),
		closed: map[*types.Var][]producer{},
		sent:   map[*types.Var][]producer{},
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.SendStmt:
				finder.record(finder.sent, stmt.Chan, stmt)
			case *ast.CallExpr:
				if builtinClose(finder.info, stmt) {
					finder.record(finder.closed, stmt.Args[0], stmt)
				}
			}
			return true
		})
	}
	finder.passDown(finder.closed)
	finder.passDown(finder.sent)
	return finder
}

func (f *Finder) passDown(into map[*types.Var][]producer) {
	work := slices.Clone(f.params)
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		param := work[i]
		if len(into[param]) == 0 {
			continue
		}
		for _, arg := range f.index.Arguments(param) {
			obj := f.index.Object(arg.Expr)
			if obj == nil || obj == param {
				continue
			}
			credited := func(p producer) bool { return p.node == arg.Expr }
			if slices.ContainsFunc(into[obj], credited) {
				continue
			}
			into[obj] = append(into[obj], producer{node: arg.Expr, decl: arg.Decl})
			if f.index.IsParameter(obj) {
				work = append(work, obj)
			}
		}
	}
	assert.Ok(i == len(work), "passDown: worklist reached worklist.NodesMax")
}

func (f *Finder) record(into map[*types.Var][]producer, expr ast.Expr, node ast.Node) {
	if obj := f.index.Object(expr); obj != nil {
		if f.index.IsParameter(obj) && len(into[obj]) == 0 && !slices.Contains(f.params, obj) {
			f.params = append(f.params, obj)
		}
		into[obj] = append(into[obj], producer{node: node, decl: f.index.DeclOf(node)})
	}
}

func builtinClose(info *types.Info, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || len(call.Args) != 1 {
		return false
	}
	_, builtin := info.Uses[ident].(*types.Builtin)
	return builtin && ident.Name == "close"
}

func (f *Finder) Case(comm ast.Stmt) bool {
	if send, ok := comm.(*ast.SendStmt); ok {
		return f.produced(send.Chan, probe{at: send, skip: send, kinds: closesOrSends})
	}
	arrow := Receive(comm)
	return arrow != nil && f.produced(arrow.X, probe{at: arrow, kinds: closesOrSends})
}

func (f *Finder) Index() *ownership.Index {
	return f.index
}

func (f *Finder) ContextCase(comm ast.Stmt) bool {
	arrow := Receive(comm)
	return arrow != nil && f.liveDone(arrow.X, f.index.DeclOf(arrow))
}

func (f *Finder) Closes(comm ast.Stmt) bool {
	arrow := Receive(comm)
	return arrow != nil && f.BareReceive(arrow)
}

func (f *Finder) BareReceive(recv *ast.UnaryExpr) bool {
	return f.produced(recv.X, probe{at: recv, kinds: closesOnly})
}

func Receive(comm ast.Stmt) *ast.UnaryExpr {
	var expr ast.Expr
	switch stmt := comm.(type) {
	case *ast.ExprStmt:
		expr = stmt.X
	case *ast.AssignStmt:
		if len(stmt.Rhs) == 1 {
			expr = stmt.Rhs[0]
		}
	}
	arrow, ok := expr.(*ast.UnaryExpr)
	if !ok || arrow.Op != token.ARROW {
		return nil
	}
	return arrow
}

type kinds int

const (
	closesOrSends kinds = iota
	closesOnly
)

type probe struct {
	at    ast.Node
	skip  ast.Node
	kinds kinds
}

type target struct {
	expr ast.Expr
	decl *ast.FuncDecl
}

func (f *Finder) produced(expr ast.Expr, ask probe) bool {
	work := []target{{expr: expr, decl: f.index.DeclOf(ask.at)}}
	seen := map[ast.Expr]bool{expr: true}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		here := work[i]
		if f.liveDone(here.expr, here.decl) {
			return true
		}
		obj := f.index.Object(here.expr)
		if obj == nil {
			continue
		}
		if f.reached(f.closed[obj], obj, here.decl, nil) {
			return true
		}
		if ask.kinds == closesOrSends && f.reached(f.sent[obj], obj, here.decl, ask.skip) {
			return true
		}
		for _, def := range f.origins(obj, here.decl) {
			if !seen[def.Expr] {
				seen[def.Expr] = true
				work = append(work, target{expr: def.Expr, decl: def.Decl})
			}
		}
	}
	assert.Ok(i == len(work), "produced: worklist reached worklist.NodesMax")
	return false
}

func (f *Finder) reached(sites []producer, obj *types.Var, use *ast.FuncDecl, skip ast.Node) bool {
	for _, site := range sites {
		seen := ownership.Link{Actor: site.decl, Reader: use}
		if site.node != skip && f.index.Reaches(obj, seen) {
			return true
		}
	}
	return false
}

func (f *Finder) origins(obj *types.Var, decl *ast.FuncDecl) []ownership.Argument {
	if f.index.IsParameter(obj) {
		return f.index.Arguments(obj)
	}
	if obj.IsField() || obj.Parent() == obj.Pkg().Scope() {
		return nil
	}
	var out []ownership.Argument
	for _, def := range f.index.Definitions(obj, decl) {
		out = append(out, ownership.Argument{Expr: def, Decl: decl})
	}
	return out
}

func (f *Finder) liveDone(expr ast.Expr, decl *ast.FuncDecl) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Done" || !isContext(f.info.TypeOf(selector.X)) {
		return false
	}
	return f.live(selector.X, decl)
}

func isContext(typ types.Type) bool {
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}

func (f *Finder) live(expr ast.Expr, decl *ast.FuncDecl) bool {
	work := []ast.Expr{expr}
	seen := map[ast.Expr]bool{expr: true}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		var next []ast.Expr
		if call, isCall := ast.Unparen(work[i]).(*ast.CallExpr); isCall {
			name, fromContext := f.contextFunc(call)
			if !fromContext {
				return true
			}
			if derivers[name] && len(call.Args) > 0 {
				next = append(next, call.Args[0])
			}
		} else {
			defs, external := f.definitions(work[i], decl)
			if external {
				return true
			}
			next = defs
		}
		for _, candidate := range next {
			if !seen[candidate] {
				seen[candidate] = true
				work = append(work, candidate)
			}
		}
	}
	assert.Ok(i == len(work), "live: worklist reached worklist.NodesMax")
	return false
}

func (f *Finder) definitions(expr ast.Expr, decl *ast.FuncDecl) ([]ast.Expr, bool) {
	obj := f.index.Object(expr)
	if obj == nil || obj.IsField() || f.index.IsParameter(obj) {
		return nil, true
	}
	if obj.Parent() == obj.Pkg().Scope() {
		return nil, true
	}
	defs := f.index.Definitions(obj, decl)
	return defs, len(defs) == 0
}

func (f *Finder) contextFunc(call *ast.CallExpr) (string, bool) {
	var ident *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return "", false
	}
	fn, ok := f.info.Uses[ident].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "context" || fn.Signature().Recv() != nil {
		return "", false
	}
	return fn.Name(), true
}
