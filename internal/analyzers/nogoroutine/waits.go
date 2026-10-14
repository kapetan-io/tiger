package nogoroutine

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/cfg"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/worklist"
)

// flowQuery asks whether a group is waited on along every success path of a
// function. With a start, only the paths after the start count, and a Wait
// before it does not. Without one, every path from the function's entry does.
type flowQuery struct {
	group *types.Var
	start *ast.SelectorExpr
	// anyPath relaxes the question to whether some path after the start waits.
	// TryGo uses it: the path where it returned false started nothing.
	anyPath bool
}

// flowState is where a walk stands in one block: whether it has passed the
// start, and whether it has passed a Wait (or taken the case where the caller
// gave up).
type flowState struct {
	block   *cfg.Block
	passed  bool
	covered bool
}

// stateKey identifies a flowState without its block pointer.
type stateKey struct {
	block   int32
	passed  bool
	covered bool
}

// waiters is a set of declarations known to wait on a group along every
// success path.
type waiters map[*ast.FuncDecl]bool

// summary returns the declarations that wait on group along every success
// path, found by repeating until no declaration is added: a method that calls
// a helper that always waits also always waits.
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
			if !always[decl] && !o.uncovered(decl, flowQuery{group: group}, always) {
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

// uncovered reports whether some success path through decl, after the query's
// start, reaches the end without a Wait on the group. The always set holds the
// helpers known to wait.
func (o *owners) uncovered(decl *ast.FuncDecl, query flowQuery, always waiters) bool {
	graph := o.graph(decl)
	if len(graph.Blocks) == 0 {
		return false
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
			return false
		}
		if len(state.block.Succs) == 0 {
			// The graph ends the chain of select cases in a block that no
			// statement follows; a select with no default never falls out there.
			if state.block.Kind == cfg.KindSelectAfterCase {
				continue
			}
			unwaited := state.passed && !state.covered && !query.anyPath
			if unwaited && o.successExit(decl, state.block) {
				return true
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
	return query.anyPath
}

// walk follows the nodes of one block. A Wait covers the path once the start
// is behind it. A deferred Wait covers the path wherever it sits, since it
// runs when the function exits. A node inside the case where the caller gave
// up covers the path too: that is the early return the rule exempts.
func (o *owners) walk(state flowState, query flowQuery, always waiters) flowState {
	for _, node := range state.block.Nodes {
		if query.start != nil && covers(node, query.start) {
			state.passed = true
		}
		if o.inExpired(node.Pos()) {
			state.covered = true
		}
		if !o.waitsIn(node, query.group, always) {
			continue
		}
		if _, deferred := node.(*ast.DeferStmt); deferred || state.passed {
			state.covered = true
		}
	}
	return state
}

// covers reports whether node spans the start.
func covers(node ast.Node, start *ast.SelectorExpr) bool {
	return node.Pos() <= start.Pos() && start.End() <= node.End()
}

// inExpired reports whether pos sits in the body of a case where the caller
// gave up waiting.
func (o *owners) inExpired(pos token.Pos) bool {
	for _, gone := range o.expired {
		if gone.from <= pos && pos < gone.to {
			return true
		}
	}
	return false
}

// waitsIn reports whether node calls Wait on group, or calls a helper known to
// wait. A function literal counts only where it is called on the spot, and
// never when it is the function a Go call starts, since that goroutine would
// be waiting on itself.
func (o *owners) waitsIn(node ast.Node, group *types.Var, always waiters) bool {
	work := []ast.Node{node}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		waits := false
		ast.Inspect(work[i], func(inner ast.Node) bool {
			switch found := inner.(type) {
			case *ast.FuncLit, *ast.GoStmt:
				return false
			case *ast.CallExpr:
				if literal, ok := ast.Unparen(found.Fun).(*ast.FuncLit); ok {
					work = append(work, literal.Body)
				}
				waits = waits || o.callWaits(found, group, always)
			}
			return !waits
		})
		if waits {
			return true
		}
	}
	assert.Ok(i == len(work), "waitsIn: worklist reached worklist.NodesMax")
	return false
}

// callWaits reports whether call is a Wait on group outside the cases where
// the caller gave up, or a call to a helper known to wait.
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

// helperWaits reports whether call invokes a package function known to wait.
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

// graph returns the control flow of decl, built once. A call to panic does not
// return.
func (o *owners) graph(decl *ast.FuncDecl) *cfg.CFG {
	if built, ok := o.graphs[decl]; ok {
		return built
	}
	built := cfg.New(decl.Body, o.mayReturn)
	o.graphs[decl] = built
	return built
}

// mayReturn reports whether a call can come back, which only panic cannot.
func (o *owners) mayReturn(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "panic" {
		return true
	}
	_, builtin := o.pass.TypesInfo.Uses[ident].(*types.Builtin)
	return !builtin
}

// successExit reports whether a path ending in block is a success path. A
// return of an error value that is not nil is the failure path, and the rule
// asks for a Wait on the success path only.
func (o *owners) successExit(decl *ast.FuncDecl, block *cfg.Block) bool {
	if len(block.Nodes) == 0 {
		return true
	}
	ret, ok := block.Nodes[len(block.Nodes)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) == 0 || decl.Type.Results == nil {
		return true
	}
	last := decl.Type.Results.List[len(decl.Type.Results.List)-1]
	if !types.Identical(o.pass.TypesInfo.TypeOf(last.Type), types.Universe.Lookup("error").Type()) {
		return true
	}
	result := ast.Unparen(ret.Results[len(ret.Results)-1])
	if _, isCall := result.(*ast.CallExpr); isCall {
		return true
	}
	ident, isIdent := result.(*ast.Ident)
	return isIdent && ident.Name == "nil"
}
