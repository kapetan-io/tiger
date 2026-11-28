package nogoroutine

import (
	"go/ast"
	"go/token"
	"go/types"
	"math"

	"golang.org/x/tools/go/cfg"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/worklist"
)

const (
	heldLocalsMax   = 4
	heldLiteralsMax = 62
	reachingBits    = 64
)

const (
	_ = uint(heldLocalsMax - 1)
	_ = uint(math.MaxUint8 - heldLiteralsMax)
	_ = uint(reachingBits - 1 - heldLiteralsMax)
)

type heldIndexes [heldLocalsMax]uint8

type followedLocal struct {
	local    *types.Var
	literals []*ast.FuncLit
}

type heldOnPath struct {
	followed []followedLocal
	holding  heldIndexes
}

func (h heldOnPath) follows(local *types.Var) bool {
	for _, followed := range h.followed {
		if followed.local == local {
			return true
		}
	}
	return false
}

func (o *owners) literalOnPath(expr ast.Expr, onPath heldOnPath) *ast.FuncLit {
	if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
		local := o.index.Object(ident)
		for i, followed := range onPath.followed {
			if local == nil || followed.local != local {
				continue
			}
			if onPath.holding[i] == 0 {
				return nil
			}
			return followed.literals[onPath.holding[i]-1]
		}
	}
	return o.literalOf(expr)
}

func (o *owners) followedLocals(fn function, group *types.Var, always waiters) []followedLocal {
	if fn.decl == nil {
		return nil
	}
	var followed []followedLocal
	ast.Inspect(fn.body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok || len(followed) == heldLocalsMax {
			return false
		}
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		local, defined := o.pass.TypesInfo.Defs[ident].(*types.Var)
		if !defined {
			return true
		}
		if literals, ok := o.followable(fn, local); ok && o.anyMayWait(literals, group, always) {
			followed = append(followed, followedLocal{local: local, literals: literals})
		}
		return true
	})
	return followed
}

func (o *owners) followable(fn function, local *types.Var) ([]*ast.FuncLit, bool) {
	if _, isFunc := local.Type().Underlying().(*types.Signature); !isFunc {
		return nil, false
	}
	if o.index.IsParameter(local) || o.rebound(local, fn.decl) {
		return nil, false
	}
	defs := o.index.Definitions(local, fn.decl)
	if len(defs) < 2 || len(defs) > heldLiteralsMax {
		return nil, false
	}
	if !o.assignedIn(local, fn.decl, fn.body, defs) {
		return nil, false
	}
	var literals []*ast.FuncLit
	for _, def := range defs {
		if literal, ok := ast.Unparen(def).(*ast.FuncLit); ok {
			literals = append(literals, literal)
		}
	}
	return literals, true
}

func (o *owners) anyMayWait(literals []*ast.FuncLit, group *types.Var, always waiters) bool {
	for _, literal := range literals {
		if o.searchFor(always).mayWait(literal.Body, group, nil) {
			return true
		}
	}
	return false
}

func (o *owners) heldAfter(
	node ast.Node, followed []followedLocal, held heldIndexes,
) heldIndexes {
	for i, local := range followed {
		for _, value := range o.assignments(node, local.local) {
			held[i] = 0
			for at, literal := range local.literals {
				if ast.Unparen(value) == literal {
					held[i] = uint8(at + 1)
				}
			}
		}
	}
	return held
}

func (o *owners) runSite(decl *ast.FuncDecl, body *ast.BlockStmt, use ast.Expr) ast.Node {
	enclosing := enclosingNodes(decl.Body, use.Pos())
	top := indexOf(enclosing, body)
	if top < 0 {
		return nil
	}
	var site ast.Node = use
	for i := len(enclosing) - 1; i > top; i-- {
		literal, ok := enclosing[i].(*ast.FuncLit)
		if !ok {
			continue
		}
		called := callOfLiteral(enclosing, i, literal)
		if called.call == nil || deferredOrGo(called) || !o.runsInPlace(called.call, literal) {
			return nil
		}
		site = called.call
	}
	return site
}

func (o *owners) runsInPlace(call *ast.CallExpr, literal *ast.FuncLit) bool {
	if ast.Unparen(call.Fun) == literal {
		return true
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	return ok && len(call.Args) == 2 && subtestRuns[o.calleeName(selector)] &&
		ast.Unparen(call.Args[1]) == literal && !o.callsParallel(literal)
}

func deferredIn(decl *ast.FuncDecl, body *ast.BlockStmt, def ast.Expr) bool {
	enclosing := enclosingNodes(decl.Body, def.Pos())
	top := indexOf(enclosing, body)
	if top < 0 {
		return false
	}
	for i := top + 1; i < len(enclosing); i++ {
		literal, ok := enclosing[i].(*ast.FuncLit)
		if !ok {
			continue
		}
		called := callOfLiteral(enclosing, i, literal)
		deferred, isDefer := called.parent.(*ast.DeferStmt)
		return isDefer && deferred.Call == called.call && ast.Unparen(called.call.Fun) == literal
	}
	return false
}

type literalCall struct {
	call   *ast.CallExpr
	parent ast.Node
}

func callOfLiteral(enclosing []ast.Node, i int, literal *ast.FuncLit) literalCall {
	at := i - 1
	for at > 0 {
		if _, paren := enclosing[at].(*ast.ParenExpr); !paren {
			break
		}
		at--
	}
	call, ok := enclosing[at].(*ast.CallExpr)
	if !ok || at == 0 {
		return literalCall{}
	}
	return literalCall{call: call, parent: enclosing[at-1]}
}

func deferredOrGo(called literalCall) bool {
	switch stmt := called.parent.(type) {
	case *ast.DeferStmt:
		return stmt.Call == called.call
	case *ast.GoStmt:
		return stmt.Call == called.call
	}
	return false
}

func enclosingNodes(root ast.Node, pos token.Pos) []ast.Node {
	var enclosing []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil || pos < node.Pos() || node.End() <= pos {
			return false
		}
		enclosing = append(enclosing, node)
		return true
	})
	return enclosing
}

func indexOf(enclosing []ast.Node, node ast.Node) int {
	for i, step := range enclosing {
		if step == node {
			return i
		}
	}
	return -1
}

func (o *owners) literalOf(expr ast.Expr) *ast.FuncLit {
	if literal, ok := ast.Unparen(expr).(*ast.FuncLit); ok {
		return literal
	}
	return o.valuesAt(expr).sole()
}

func (o *owners) localFunc(expr ast.Expr) *types.Var {
	expr = ast.Unparen(expr)
	local := o.index.Object(expr)
	if _, isIdent := expr.(*ast.Ident); !isIdent || local == nil || local.IsField() {
		return nil
	}
	if o.index.IsParameter(local) || local.Parent() == local.Pkg().Scope() {
		return nil
	}
	decl := o.index.DeclOf(expr)
	if decl == nil || o.rebound(local, decl) {
		return nil
	}
	return local
}

func (o *owners) soleDefinition(expr ast.Expr) ast.Expr {
	local := o.localFunc(expr)
	if local == nil {
		return nil
	}
	defs := o.index.Definitions(local, o.index.DeclOf(expr))
	if len(defs) != 1 {
		return nil
	}
	return ast.Unparen(defs[0])
}

type heldValues struct {
	literals []*ast.FuncLit
	other    bool
}

func (h heldValues) sole() *ast.FuncLit {
	if h.other || len(h.literals) != 1 {
		return nil
	}
	return h.literals[0]
}

func (o *owners) valuesAt(use ast.Expr) heldValues {
	local := o.localFunc(use)
	if local == nil {
		return heldValues{other: true}
	}
	decl := o.index.DeclOf(use)
	defs := o.index.Definitions(local, decl)
	if len(defs) == 1 {
		literal, ok := ast.Unparen(defs[0]).(*ast.FuncLit)
		if !ok {
			return heldValues{other: true}
		}
		return heldValues{literals: []*ast.FuncLit{literal}}
	}
	return o.reachingValues(local, use, decl, defs)
}

type reaching uint64

const otherValue reaching = 1 << (reachingBits - 1)

func (o *owners) reachingValues(
	local *types.Var, use ast.Expr, decl *ast.FuncDecl, defs []ast.Expr,
) heldValues {
	body := innermostBody(decl, local.Pos())
	site := o.runSite(decl, body, use)
	if site == nil || len(defs) > heldLiteralsMax || !o.assignedIn(local, decl, body, defs) {
		return heldValues{other: true}
	}
	bit := func(value ast.Expr) reaching {
		for i, def := range defs {
			if def == value {
				return 1 << i
			}
		}
		return otherValue
	}
	graph := o.graph(body)
	entry := map[*cfg.Block]reaching{graph.Blocks[0]: 0}
	work := []*cfg.Block{graph.Blocks[0]}
	var found reaching
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		block := work[i]
		state := entry[block]
		for _, node := range block.Nodes {
			if node.Pos() <= site.Pos() && site.End() <= node.End() {
				found |= state
			}
			for _, value := range o.assignments(node, local) {
				state = bit(value)
			}
		}
		for _, next := range block.Succs {
			before, seen := entry[next]
			if !seen || before|state != before {
				entry[next] = before | state
				work = append(work, next)
			}
		}
	}
	assert.Ok(i == len(work), "reachingValues: worklist reached worklist.NodesMax")
	held := heldValues{other: found == 0 || found&otherValue != 0}
	for i, def := range defs {
		if found&(1<<i) == 0 {
			continue
		}
		literal, ok := ast.Unparen(def).(*ast.FuncLit)
		if !ok {
			held.other = true
			continue
		}
		held.literals = append(held.literals, literal)
	}
	return held
}

func (o *owners) assignedIn(
	local *types.Var, decl *ast.FuncDecl, body *ast.BlockStmt, defs []ast.Expr,
) bool {
	placed := map[ast.Expr]bool{}
	for _, block := range o.graph(body).Blocks {
		for _, node := range block.Nodes {
			for _, value := range o.assignments(node, local) {
				placed[value] = true
			}
		}
	}
	for _, def := range defs {
		if !placed[def] && !deferredIn(decl, body, def) {
			return false
		}
	}
	return true
}

func (o *owners) assignments(node ast.Node, local *types.Var) []ast.Expr {
	var out []ast.Expr
	assign := func(targets, values []ast.Expr) {
		for i, target := range targets {
			if o.index.Object(target) != local {
				continue
			}
			switch {
			case len(targets) == len(values):
				out = append(out, values[i])
			case len(values) == 0:
				out = append(out, nil)
			default:
				out = append(out, values[0])
			}
		}
	}
	ast.Inspect(node, func(inner ast.Node) bool {
		switch stmt := inner.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE {
				assign(stmt.Lhs, stmt.Rhs)
			}
		case *ast.ValueSpec:
			names := make([]ast.Expr, len(stmt.Names))
			for i, name := range stmt.Names {
				names[i] = name
			}
			assign(names, stmt.Values)
		}
		return true
	})
	return out
}

func innermostBody(decl *ast.FuncDecl, pos token.Pos) *ast.BlockStmt {
	body := decl.Body
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		literal, ok := node.(*ast.FuncLit)
		if !ok {
			return true
		}
		if pos < literal.Pos() || literal.End() <= pos {
			return false
		}
		body = literal.Body
		return true
	})
	return body
}
