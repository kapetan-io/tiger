// Package stopcase recognizes the case of a select that can end a wait,
// shared by boundedloop (TS-S03, TS-S23), selectctx (TS-C05) and nogoroutine
// (TS-C02, for the expired-context early return).
//
// A case is a stop case in one of two forms. The first is a receive from
// ctx.Done() on a context.Context that does not trace, within the function,
// to context.Background() or context.TODO(): a context made from nothing in
// the same function is cancelled only by that function, so waiting on it
// proves nothing about shutdown. The second is a receive or send on a channel
// the package itself closes or sends on, where the close or send sits in code
// the package's exported surface reaches (see internal/analyzers/internal/
// ownership). A channel's name and its element type never count, because a
// name is a claim and nothing makes the channel fire.
//
// Known misses: a close in dead code that a reachable function still names
// counts, and a channel the package never closes but a caller does is not
// recognized.
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

// derivers are the context functions whose result is cancelled when its first
// argument is.
var derivers = map[string]bool{
	"WithCancel": true, "WithCancelCause": true, "WithTimeout": true, "WithTimeoutCause": true,
	"WithDeadline": true, "WithDeadlineCause": true, "WithValue": true,
}

// producer is one close or send on a channel.
type producer struct {
	node ast.Node
	decl *ast.FuncDecl
}

// Finder recognizes stop cases in one package.
type Finder struct {
	info   *types.Info
	index  *ownership.Index
	closed map[*types.Var][]producer
	sent   map[*types.Var][]producer
	params []*types.Var
}

// New indexes where the package closes and sends on channels.
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

// passDown credits a close or send on a parameter to the channels the package
// passes for it, at the call that passes them, so a helper like shut(ch) that
// closes its argument counts for the field its callers hand it.
func (f *Finder) passDown(into map[*types.Var][]producer) {
	for _, param := range f.params {
		if len(into[param]) == 0 {
			continue
		}
		for _, arg := range f.index.Arguments(param) {
			if obj := f.index.Object(arg.Expr); obj != nil && obj != param {
				into[obj] = append(into[obj], producer{node: arg.Expr, decl: arg.Decl})
			}
		}
	}
}

// record notes node as a producer for the channel expr names.
func (f *Finder) record(into map[*types.Var][]producer, expr ast.Expr, node ast.Node) {
	if obj := f.index.Object(expr); obj != nil {
		if f.index.IsParameter(obj) && len(into[obj]) == 0 && !slices.Contains(f.params, obj) {
			f.params = append(f.params, obj)
		}
		into[obj] = append(into[obj], producer{node: node, decl: f.index.DeclOf(node)})
	}
}

// builtinClose reports whether call is the builtin close with an argument.
func builtinClose(info *types.Info, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || len(call.Args) != 1 {
		return false
	}
	_, builtin := info.Uses[ident].(*types.Builtin)
	return builtin && ident.Name == "close"
}

// Case reports whether comm, a select case's headline statement, is a stop
// case: a receive from a live ctx.Done(), or a receive or send on a channel
// the package produces on. A nil comm is a default case and is not one.
func (f *Finder) Case(comm ast.Stmt) bool {
	if send, ok := comm.(*ast.SendStmt); ok {
		return f.produced(send.Chan, probe{at: send, skip: send, kinds: closesOrSends})
	}
	arrow := Receive(comm)
	return arrow != nil && f.produced(arrow.X, probe{at: arrow, kinds: closesOrSends})
}

// Index returns the ownership index the finder was built on.
func (f *Finder) Index() *ownership.Index {
	return f.index
}

// ContextCase reports whether comm, a select case's headline statement,
// receives from a live ctx.Done(). In a function that waits for work, such a
// case is the caller giving up, not the work finishing.
func (f *Finder) ContextCase(comm ast.Stmt) bool {
	arrow := Receive(comm)
	return arrow != nil && f.liveDone(arrow.X, f.index.DeclOf(arrow))
}

// Closes reports whether comm, a select case's headline statement, receives
// from a live ctx.Done() or a channel the package closes. A channel the
// package only sends on may carry shutdown requests or ordinary work, and a
// case on it cannot tell the two apart.
func (f *Finder) Closes(comm ast.Stmt) bool {
	arrow := Receive(comm)
	return arrow != nil && f.BareReceive(arrow)
}

// BareReceive reports whether a receive outside a select is itself a wait for
// shutdown: from a live ctx.Done(), or from a channel the package closes. A
// channel the package only sends on is a reply wait, and a reply that never
// comes is the bug TS-C05 exists to catch.
func (f *Finder) BareReceive(recv *ast.UnaryExpr) bool {
	return f.produced(recv.X, probe{at: recv, kinds: closesOnly})
}

// Receive extracts the receive expression from a select case's headline
// statement, covering "<-ch", "v := <-ch" and "v, ok := <-ch". It returns nil
// for a send, a default case, and anything else.
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

// kinds says which operations on a channel make a case a stop case.
type kinds int

const (
	closesOrSends kinds = iota
	closesOnly
)

// probe is a question about one channel use: where it sits, which operation
// is being judged (it is not evidence for itself), and which operations by the
// package count.
type probe struct {
	at    ast.Node
	skip  ast.Node
	kinds kinds
}

// target is an expression to judge and the declaration it was written in.
type target struct {
	expr ast.Expr
	decl *ast.FuncDecl
}

// produced reports whether expr, a channel used where the probe says, is a
// live ctx.Done() or a channel the package closes (or sends on, unless the
// probe asks for closes only). It follows a local to the expressions assigned
// to it and a parameter to the arguments the package passes.
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

// reached reports whether any of sites, other than skip, is code the package's
// exported surface reaches for obj as read in use.
func (f *Finder) reached(sites []producer, obj *types.Var, use *ast.FuncDecl, skip ast.Node) bool {
	for _, site := range sites {
		seen := ownership.Link{Actor: site.decl, Reader: use}
		if site.node != skip && f.index.Reaches(obj, seen) {
			return true
		}
	}
	return false
}

// origins returns where a channel variable's value comes from: the argument
// each call passes when it is a parameter, or the expression assigned to it
// when it is a local.
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

// liveDone reports whether expr is a call to Done on a context.Context that is
// not cancelled only by the function holding it.
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

// isContext reports whether typ is context.Context.
func isContext(typ types.Type) bool {
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}

// live reports whether a context expression may be cancelled by someone other
// than the function holding it. A context that traces to Background or TODO
// through the function's own locals is not; a parameter, field or package
// variable is, because the caller or owner decides.
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

// definitions returns the expressions assigned to the local variable expr
// names. It reports external when the value comes from outside the function:
// a field, a package variable, a parameter, something unnamed, or a local that
// nothing assigns.
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

// contextFunc returns the name of the context package function call invokes.
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
