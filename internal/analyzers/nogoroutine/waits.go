package nogoroutine

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/cfg"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/worklist"
)

type flowQuery struct {
	group   *types.Var
	start   *ast.SelectorExpr
	anyPath bool
}

type flowState struct {
	block   *cfg.Block
	passed  bool
	covered bool
}

type stateKey struct {
	block   int32
	passed  bool
	covered bool
}

type waiters map[*ast.FuncDecl]bool

type function struct {
	decl *ast.FuncDecl
	typ  *ast.FuncType
	body *ast.BlockStmt
}

func functionOf(decl *ast.FuncDecl) function {
	return function{decl: decl, typ: decl.Type, body: decl.Body}
}

func (o *owners) summary(group *types.Var) waiters {
	if known, ok := o.summaries[group]; ok {
		return known
	}
	always := waiters{}
	decls := o.index.Decls()
	settled := false
	for round := 0; round <= len(decls); round++ {
		changed := false
		for _, decl := range decls {
			if always[decl] {
				continue
			}
			unwaited, _ := o.uncovered(functionOf(decl), flowQuery{group: group}, always)
			if !unwaited {
				always[decl] = true
				changed = true
			}
		}
		if !changed {
			settled = true
			break
		}
	}
	assert.Ok(settled, "summary: no fixpoint within len(decls)+1 rounds")
	o.summaries[group] = always
	return always
}

func (o *owners) uncovered(fn function, query flowQuery, always waiters) (unwaited, exits bool) {
	graph := o.graph(fn.body)
	if len(graph.Blocks) == 0 {
		return false, false
	}
	seen := map[stateKey]bool{}
	work := []flowState{{block: graph.Blocks[0], passed: query.start == nil}}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		state := work[i]
		key := stateKey{block: state.block.Index, passed: state.passed, covered: state.covered}
		if seen[key] {
			continue
		}
		seen[key] = true
		state = o.walk(state, query, always)
		if query.anyPath && state.passed && state.covered {
			return false, true
		}
		if len(state.block.Succs) == 0 {
			if state.block.Kind == cfg.KindSelectAfterCase {
				continue
			}
			if !state.passed || !o.successExit(fn, state.block) {
				continue
			}
			exits = true
			if !state.covered && !query.anyPath {
				return true, true
			}
			continue
		}
		for _, next := range state.block.Succs {
			work = append(work, flowState{
				block: next, passed: state.passed, covered: state.covered,
			})
		}
	}
	assert.Ok(i == len(work), "uncovered: worklist reached worklist.NodesMax")
	return query.anyPath, exits
}

func (o *owners) walk(state flowState, query flowQuery, always waiters) flowState {
	for _, node := range state.block.Nodes {
		if query.start != nil && covers(node, query.start) {
			state.passed = true
		}
		if o.inExpired(node.Pos()) {
			state.covered = true
		}
		if !o.waitsIn(node, query, always) {
			continue
		}
		if _, deferred := node.(*ast.DeferStmt); deferred || state.passed {
			state.covered = true
		}
	}
	return state
}

func covers(node ast.Node, start *ast.SelectorExpr) bool {
	return node.Pos() <= start.Pos() && start.End() <= node.End()
}

func (o *owners) inExpired(pos token.Pos) bool {
	for _, gone := range o.expired {
		if gone.from <= pos && pos < gone.to {
			return true
		}
	}
	return false
}

func (o *owners) waitsIn(node ast.Node, query flowQuery, always waiters) bool {
	waits := false
	ast.Inspect(node, func(inner ast.Node) bool {
		switch found := inner.(type) {
		case *ast.FuncLit, *ast.GoStmt:
			return false
		case *ast.CallExpr:
			waits = o.callWaits(found, query.group, always)
			if literal := o.runLiteral(found); !waits && literal != nil {
				waits = o.literalWaits(literal, query, always)
			}
		}
		return !waits
	})
	return waits
}

func (o *owners) literalWaits(literal *ast.FuncLit, query flowQuery, always waiters) bool {
	if o.following[literal] {
		return false
	}
	if query.start != nil && covers(literal, query.start) {
		return false
	}
	o.following[literal] = true
	defer delete(o.following, literal)
	fn := function{decl: o.index.DeclOf(literal), typ: literal.Type, body: literal.Body}
	unwaited, _ := o.uncovered(fn, flowQuery{group: query.group}, always)
	return !unwaited
}

func (o *owners) runLiteral(call *ast.CallExpr) *ast.FuncLit {
	if literal := o.literalOf(call.Fun); literal != nil {
		return literal
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 2 || !subtestRuns[o.methodName(selector)] {
		return nil
	}
	body := o.literalOf(call.Args[1])
	if body == nil || o.callsParallel(body) {
		return nil
	}
	return body
}

var subtestRuns = map[string]bool{"(*testing.T).Run": true, "(*testing.B).Run": true}

func (o *owners) literalOf(expr ast.Expr) *ast.FuncLit {
	expr = ast.Unparen(expr)
	if literal, ok := expr.(*ast.FuncLit); ok {
		return literal
	}
	local := o.index.Object(expr)
	if _, isIdent := expr.(*ast.Ident); !isIdent || local == nil || local.IsField() {
		return nil
	}
	if o.index.IsParameter(local) || local.Parent() == local.Pkg().Scope() {
		return nil
	}
	defs := o.index.Definitions(local, o.index.DeclOf(expr))
	if len(defs) != 1 {
		return nil
	}
	literal, _ := ast.Unparen(defs[0]).(*ast.FuncLit)
	return literal
}

func (o *owners) callsParallel(literal *ast.FuncLit) bool {
	parallel := false
	ast.Inspect(literal.Body, func(node ast.Node) bool {
		switch found := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			selector, ok := ast.Unparen(found.Fun).(*ast.SelectorExpr)
			parallel = parallel || ok && o.methodName(selector) == "(*testing.T).Parallel"
		}
		return !parallel
	})
	return parallel
}

func (o *owners) methodName(selector *ast.SelectorExpr) string {
	fn, ok := o.pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok {
		return ""
	}
	return fn.FullName()
}

func (o *owners) callWaits(call *ast.CallExpr, group *types.Var, always waiters) bool {
	if o.inExpired(call.Pos()) {
		return false
	}
	if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if waited, isWait := o.groupMethod(selector, "Wait"); isWait && waited == group {
			return true
		}
	}
	return o.helperWaits(call, always)
}

func (o *owners) helperWaits(call *ast.CallExpr, always waiters) bool {
	var ident *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return false
	}
	fn, ok := o.pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok {
		return false
	}
	decl := o.index.DeclFor(fn)
	return decl != nil && always[decl]
}

func (o *owners) graph(body *ast.BlockStmt) *cfg.CFG {
	if built, ok := o.graphs[body]; ok {
		return built
	}
	built := cfg.New(body, o.mayReturn)
	o.graphs[body] = built
	return built
}

func (o *owners) mayReturn(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "panic" {
		return true
	}
	_, builtin := o.pass.TypesInfo.Uses[ident].(*types.Builtin)
	return !builtin
}

func (o *owners) successExit(fn function, block *cfg.Block) bool {
	if len(block.Nodes) == 0 {
		return true
	}
	ret, ok := block.Nodes[len(block.Nodes)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) == 0 || fn.typ.Results == nil {
		return true
	}
	last := fn.typ.Results.List[len(fn.typ.Results.List)-1]
	if !types.Identical(o.pass.TypesInfo.TypeOf(last.Type), types.Universe.Lookup("error").Type()) {
		return true
	}
	result := ast.Unparen(ret.Results[len(ret.Results)-1])
	if _, isCall := result.(*ast.CallExpr); isCall {
		return true
	}
	ident, isIdent := result.(*ast.Ident)
	if !isIdent {
		return false
	}
	return ident.Name == "nil" || o.unsetResult(fn, ident)
}

func (o *owners) unsetResult(fn function, ident *ast.Ident) bool {
	result := o.index.Object(ident)
	if result == nil || fn.typ.Results == nil {
		return false
	}
	named := fn.typ.Results
	if result.Pos() < named.Pos() || named.End() < result.Pos() {
		return false
	}
	for _, source := range o.index.Definitions(result, fn.decl) {
		if !o.pass.TypesInfo.Types[source].IsNil() {
			return false
		}
	}
	return true
}
