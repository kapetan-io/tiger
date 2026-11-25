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
	group    *types.Var
	start    *ast.SelectorExpr
	held     *ast.SelectorExpr
	anyPath  bool
	shutdown bool
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
			if always[decl] || !o.mayWait(decl.Body, group, always, searchedBodies{}) {
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

type searchedBody struct {
	body  *ast.BlockStmt
	group *types.Var
}

type searchedBodies map[searchedBody]bool

func (o *owners) mayWait(
	body *ast.BlockStmt, group *types.Var, always waiters, seen searchedBodies,
) bool {
	key := searchedBody{body: body, group: group}
	if seen[key] {
		return false
	}
	seen[key] = true
	found := false
	for _, block := range o.graph(body).Blocks {
		if !block.Live {
			continue
		}
		for _, node := range block.Nodes {
			ast.Inspect(node, func(inner ast.Node) bool {
				switch expr := inner.(type) {
				case *ast.FuncLit, *ast.GoStmt:
					return false
				case *ast.CallExpr:
					found = found || o.callMayWait(expr, group, always, seen)
				}
				return !found
			})
			if found {
				return true
			}
		}
	}
	return false
}

func (o *owners) callMayWait(
	call *ast.CallExpr, group *types.Var, always waiters, seen searchedBodies,
) bool {
	if o.callsWait(call.Fun, group) || o.helperWaits(call, always) {
		return true
	}
	decl := o.calledDecl(call)
	if bound, ok := o.soleDefinition(call.Fun).(*ast.SelectorExpr); ok && decl == nil {
		decl = o.calledDecl(&ast.CallExpr{Fun: bound})
	}
	if decl != nil && o.mayWait(decl.Body, group, always, seen) {
		return true
	}
	literal := o.literalRunBy(call)
	if literal == nil {
		literal = o.onceLiteral(call)
	}
	if literal != nil && o.mayWait(literal.Body, group, always, seen) {
		return true
	}
	callee := o.pass.TypesInfo.Types[ast.Unparen(call.Fun)]
	if callee.IsBuiltin() || callee.IsType() {
		return false
	}
	for i, arg := range call.Args {
		if o.names(arg, group) && o.handedWait(call, i, seen) {
			return true
		}
	}
	return false
}

func (o *owners) callsWait(fun ast.Expr, group *types.Var) bool {
	selector, ok := ast.Unparen(fun).(*ast.SelectorExpr)
	if !ok {
		selector, ok = o.soleDefinition(fun).(*ast.SelectorExpr)
	}
	if !ok {
		return false
	}
	waited, isWait := o.groupMethod(selector, "Wait")
	return isWait && waited == group
}

func (o *owners) handedWait(call *ast.CallExpr, arg int, seen searchedBodies) bool {
	decl := o.calledDecl(call)
	if decl == nil {
		return true
	}
	selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if isSelector && o.pass.TypesInfo.Types[selector.X].IsType() {
		arg--
	}
	params := paramNames(decl)
	if decl.Type.Params != nil && arg >= len(params)-1 && o.variadic(decl) {
		return true
	}
	if arg < 0 || arg >= len(params) || params[arg] == nil {
		return false
	}
	return o.mayWait(decl.Body, o.index.Object(params[arg]), waiters{}, seen)
}

func (o *owners) variadic(decl *ast.FuncDecl) bool {
	fn, ok := o.pass.TypesInfo.Defs[decl.Name].(*types.Func)
	return ok && fn.Signature().Variadic()
}

func (o *owners) onceLiteral(call *ast.CallExpr) *ast.FuncLit {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 1 || o.calleeName(selector) != "(*sync.Once).Do" {
		return nil
	}
	return o.literalOf(call.Args[0])
}

func (o *owners) names(arg ast.Expr, group *types.Var) bool {
	arg = ast.Unparen(arg)
	if address, ok := arg.(*ast.UnaryExpr); ok && address.Op == token.AND {
		arg = ast.Unparen(address.X)
	}
	return o.unalias(o.index.Object(arg), arg) == group
}

func (o *owners) uncovered(fn function, query flowQuery, always waiters) (unwaited, exits bool) {
	graph := o.graph(fn.body)
	if len(graph.Blocks) == 0 {
		return false, false
	}
	query.shutdown = o.givesUpIn(fn.body) && o.callsIn(fn.body, func(call *ast.CallExpr) bool {
		return o.waitCall(call, query.group, always)
	})
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
		if query.shutdown && state.passed && o.givesUp(node.Pos(), query.start) {
			state.covered = true
		}
		if query.start != nil && covers(node, query.start) {
			state.passed = true
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

func (o *owners) givesUp(pos token.Pos, start *ast.SelectorExpr) bool {
	gives := false
	for _, gone := range o.expired {
		if !gone.holds(pos) {
			continue
		}
		if start != nil && gone.holds(start.Pos()) {
			return false
		}
		gives = true
	}
	return gives
}

func (o *owners) givesUpIn(body *ast.BlockStmt) bool {
	for _, gone := range o.expired {
		if body.Pos() <= gone.from && gone.to <= body.End() {
			return true
		}
	}
	return false
}

func (o *owners) waitsIn(node ast.Node, query flowQuery, always waiters) bool {
	start := query.start
	if start == nil {
		start = query.held
	}
	waits := false
	ast.Inspect(node, func(inner ast.Node) bool {
		switch found := inner.(type) {
		case *ast.FuncLit, *ast.GoStmt:
			return false
		case *ast.CallExpr:
			counted := !o.givesUp(found.Pos(), start)
			waits = waits || counted && o.waitCall(found, query.group, always)
			if literal := o.literalRunBy(found); !waits && literal != nil {
				waits = o.literalWaits(literal, query, always)
			}
		}
		return !waits
	})
	return waits
}

func (o *owners) literalWaits(literal *ast.FuncLit, query flowQuery, always waiters) bool {
	if o.walking[literal] {
		return false
	}
	held := query.held
	if query.start != nil {
		held = query.start
	}
	if held != nil && covers(literal, held) {
		return false
	}
	o.walking[literal] = true
	defer delete(o.walking, literal)
	fn := function{decl: o.index.DeclOf(literal), typ: literal.Type, body: literal.Body}
	unwaited, _ := o.uncovered(fn, flowQuery{group: query.group, held: held}, always)
	return !unwaited
}

func (o *owners) literalRunBy(call *ast.CallExpr) *ast.FuncLit {
	if literal := o.literalOf(call.Fun); literal != nil {
		return literal
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 2 || !subtestRuns[o.calleeName(selector)] {
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
	if literal, ok := ast.Unparen(expr).(*ast.FuncLit); ok {
		return literal
	}
	if literal, ok := o.soleDefinition(expr).(*ast.FuncLit); ok {
		return literal
	}
	return nil
}

func (o *owners) soleDefinition(expr ast.Expr) ast.Expr {
	expr = ast.Unparen(expr)
	local := o.index.Object(expr)
	if _, isIdent := expr.(*ast.Ident); !isIdent || local == nil || local.IsField() {
		return nil
	}
	if o.index.IsParameter(local) || local.Parent() == local.Pkg().Scope() {
		return nil
	}
	decl := o.index.DeclOf(expr)
	defs := o.index.Definitions(local, decl)
	if len(defs) != 1 || o.rebound(local, decl) {
		return nil
	}
	return ast.Unparen(defs[0])
}

func (o *owners) rebound(local *types.Var, decl *ast.FuncDecl) bool {
	found := false
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.UnaryExpr:
			found = found || stmt.Op == token.AND && o.index.Object(stmt.X) == local
		case *ast.RangeStmt:
			found = found || o.rangeTarget(stmt.Key, local) || o.rangeTarget(stmt.Value, local)
		}
		return !found
	})
	return found
}

func (o *owners) rangeTarget(target ast.Expr, local *types.Var) bool {
	return target != nil && o.index.Object(target) == local
}

func (o *owners) callsParallel(literal *ast.FuncLit) bool {
	params := literal.Type.Params.List
	if len(params) == 0 || len(params[0].Names) == 0 {
		return false
	}
	return o.parallelOn(literal.Body, o.index.Object(params[0].Names[0]), searched{})
}

type searched map[*types.Var]bool

func (o *owners) parallelOn(body *ast.BlockStmt, subtest *types.Var, seen searched) bool {
	if subtest == nil || seen[subtest] {
		return false
	}
	seen[subtest] = true
	parallel := false
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			parallel = parallel || o.marksParallel(call, subtest, seen)
		}
		return !parallel
	})
	return parallel
}

func (o *owners) marksParallel(call *ast.CallExpr, subtest *types.Var, seen searched) bool {
	selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if isSelector && o.calleeName(selector) == "(*testing.T).Parallel" {
		if o.promoted(selector) {
			return o.embeds(selector, subtest)
		}
		return o.unalias(o.index.Object(selector.X), selector) == subtest
	}
	decl := o.calledDecl(call)
	if decl == nil {
		return false
	}
	args := call.Args
	if isSelector && o.pass.TypesInfo.Types[selector.X].IsType() && len(args) > 0 {
		args = args[1:]
	}
	params := paramNames(decl)
	for i, arg := range args {
		if i >= len(params) || params[i] == nil || o.index.Object(arg) != subtest {
			continue
		}
		if o.parallelOn(decl.Body, o.index.Object(params[i]), seen) {
			return true
		}
	}
	return false
}

func (o *owners) promoted(selector *ast.SelectorExpr) bool {
	selection := o.pass.TypesInfo.Selections[selector]
	return selection != nil && len(selection.Index()) > 1
}

func (o *owners) embeds(selector *ast.SelectorExpr, subtest *types.Var) bool {
	holder := o.unalias(o.index.Object(selector.X), selector)
	if holder == nil || holder.IsField() || o.index.IsParameter(holder) {
		return false
	}
	found := false
	for _, def := range o.index.Definitions(holder, o.index.DeclOf(selector)) {
		source := ast.Unparen(def)
		if address, ok := source.(*ast.UnaryExpr); ok && address.Op == token.AND {
			source = ast.Unparen(address.X)
		}
		if literal, ok := source.(*ast.CompositeLit); ok {
			found = found || o.holds(literal, subtest, selector)
		}
	}
	return found
}

func (o *owners) holds(literal *ast.CompositeLit, subtest *types.Var, at ast.Node) bool {
	for _, element := range literal.Elts {
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			element = pair.Value
		}
		if o.unalias(o.index.Object(element), at) == subtest {
			return true
		}
	}
	return false
}

func (o *owners) calledDecl(call *ast.CallExpr) *ast.FuncDecl {
	fun := ast.Unparen(call.Fun)
	switch generic := fun.(type) {
	case *ast.IndexExpr:
		fun = generic.X
	case *ast.IndexListExpr:
		fun = generic.X
	}
	var ident *ast.Ident
	switch named := fun.(type) {
	case *ast.Ident:
		ident = named
	case *ast.SelectorExpr:
		ident = named.Sel
	default:
		return nil
	}
	fn, ok := o.pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok {
		return nil
	}
	return o.index.DeclFor(fn)
}

func paramNames(decl *ast.FuncDecl) []*ast.Ident {
	var names []*ast.Ident
	for _, field := range decl.Type.Params.List {
		if len(field.Names) == 0 {
			names = append(names, nil)
		}
		names = append(names, field.Names...)
	}
	return names
}

func (o *owners) calleeName(selector *ast.SelectorExpr) string {
	fn, ok := o.pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok {
		return ""
	}
	return fn.FullName()
}

func (o *owners) callsIn(node ast.Node, match func(*ast.CallExpr) bool) bool {
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
				waits = waits || match(found)
			}
			return !waits
		})
		if waits {
			return true
		}
	}
	assert.Ok(i == len(work), "callsIn: worklist reached worklist.NodesMax")
	return false
}

func (o *owners) waitCall(call *ast.CallExpr, group *types.Var, always waiters) bool {
	if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if waited, isWait := o.groupMethod(selector, "Wait"); isWait && waited == group {
			return true
		}
	}
	return o.helperWaits(call, always)
}

func (o *owners) helperWaits(call *ast.CallExpr, always waiters) bool {
	decl := o.calledDecl(call)
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
