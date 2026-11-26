package nogoroutine

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/cfg"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/worklist"
)

type flowQuery struct {
	group       *types.Var
	start       *ast.SelectorExpr
	held        *ast.SelectorExpr
	anyPath     bool
	shutdown    bool
	failNowEnds bool
	subtests    []*types.Var

	errorReturnsNeedWait bool
}

type flowState struct {
	block     *cfg.Block
	passed    bool
	covered   bool
	failed    bool
	held      heldIndexes
	unchecked literalError
	nonNil    nonNilError
}

type stateKey struct {
	block     int32
	passed    bool
	covered   bool
	unchecked literalError
	nonNil    nonNilError
	held      heldIndexes
}

type literalError struct {
	call  *ast.CallExpr
	bound *types.Var
	alias *types.Var
}

type nonNilError struct {
	bound   *types.Var
	named   *types.Var
	wrapped *types.Var
}

type literalWait int

const (
	givesNoWait literalWait = iota
	errorsBeforeWait
	waitsOnEveryPath
)

type condition struct {
	expr    ast.Expr
	outcome bool
}

type waiters map[*ast.FuncDecl]bool

type function struct {
	decl     *ast.FuncDecl
	typ      *ast.FuncType
	body     *ast.BlockStmt
	followed []followedLocal
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
			if always[decl] || !o.searchFor(always).mayWait(decl.Body, group, nil) {
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
	body     *ast.BlockStmt
	group    *types.Var
	subtests string
}

type searchedBodies map[searchedBody]bool

func subtestKey(subtests []*types.Var) string {
	var key strings.Builder
	for _, subtest := range subtests {
		fmt.Fprintf(&key, "%p,", subtest)
	}
	return key.String()
}

type waitSearch struct {
	*owners
	always waiters
	seen   searchedBodies
}

func (o *owners) searchFor(always waiters) waitSearch {
	return waitSearch{owners: o, always: always, seen: searchedBodies{}}
}

func (w waitSearch) mayWait(body *ast.BlockStmt, group *types.Var, subtests []*types.Var) bool {
	key := searchedBody{body: body, group: group, subtests: subtestKey(subtests)}
	if w.seen[key] {
		return false
	}
	w.seen[key] = true
	found := false
	for _, block := range w.graph(body).Blocks {
		if !block.Live {
			continue
		}
		for _, node := range block.Nodes {
			ast.Inspect(node, func(inner ast.Node) bool {
				switch expr := inner.(type) {
				case *ast.FuncLit, *ast.GoStmt:
					return false
				case *ast.CallExpr:
					found = found || w.callMayWait(expr, group, subtests)
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

func (w waitSearch) callMayWait(call *ast.CallExpr, group *types.Var, subtests []*types.Var) bool {
	if w.callsWait(call.Fun, group) || w.helperWaits(call, w.always) {
		return true
	}
	decl := w.calledDecl(call)
	if bound, ok := w.soleDefinition(call.Fun).(*ast.SelectorExpr); ok && decl == nil {
		decl = w.calledDecl(&ast.CallExpr{Fun: bound})
	}
	if decl != nil && w.mayWait(decl.Body, group, nil) {
		return true
	}
	run := w.literalRunBy(call, subtests, heldOnPath{})
	if run.literal != nil && w.mayWait(run.literal.Body, group, run.within(subtests)) {
		return true
	}
	if saved := w.savedIn(call.Fun); run.literal == nil && saved != nil {
		for _, literal := range w.valuesAt(call.Fun).literals {
			if w.mayWait(literal.Body, group, declaredAround(subtests, saved)) {
				return true
			}
		}
	}
	once := w.onceLiteral(call)
	if once != nil && w.mayWait(once.Body, group, subtests) {
		return true
	}
	callee := w.pass.TypesInfo.Types[ast.Unparen(call.Fun)]
	if callee.IsBuiltin() || callee.IsType() {
		return false
	}
	for i, arg := range call.Args {
		if w.names(arg, group) && w.handedWait(call, i) {
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

func (w waitSearch) handedWait(call *ast.CallExpr, arg int) bool {
	decl := w.calledDecl(call)
	if decl == nil {
		return true
	}
	selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if isSelector && w.pass.TypesInfo.Types[selector.X].IsType() {
		arg--
	}
	params := paramNames(decl)
	if decl.Type.Params != nil && arg >= len(params)-1 && w.variadic(decl) {
		return true
	}
	if arg < 0 || arg >= len(params) || params[arg] == nil {
		return false
	}
	handed := waitSearch{owners: w.owners, always: waiters{}, seen: w.seen}
	return handed.mayWait(decl.Body, w.index.Object(params[arg]), nil)
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
	fn.followed = o.followedLocals(fn, query.group, always)
	walked, err := o.coverage(fn, graph, query, always)
	if err != nil {
		fn.followed = nil
		walked, err = o.coverage(fn, graph, query, always)
	}
	assert.Ok(err == nil, "uncovered: worklist reached worklist.NodesMax")
	return walked.unwaited, walked.exits
}

var errWorklistFull = errors.New("worklist reached worklist.NodesMax")

type coverage struct {
	unwaited bool
	exits    bool
}

func (o *owners) coverage(
	fn function, graph *cfg.CFG, query flowQuery, always waiters,
) (coverage, error) {
	exits := false
	seen := map[stateKey]bool{}
	work := []flowState{{block: graph.Blocks[0], passed: query.start == nil}}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		state := work[i]
		key := stateKey{
			block: state.block.Index, passed: state.passed, covered: state.covered,
			unchecked: state.unchecked, nonNil: state.nonNil, held: state.held,
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		state = o.walk(fn, state, query, always)
		if state.failed {
			exits = true
			continue
		}
		if query.anyPath && state.passed && state.covered {
			return coverage{exits: true}, nil
		}
		if len(state.block.Succs) == 0 {
			if state.block.Kind == cfg.KindSelectAfterCase {
				continue
			}
			failure := !o.successExit(fn, state.block) ||
				o.returnsNonNilError(fn, state.block, state.nonNil)
			excused := failure && !query.errorReturnsNeedWait
			if !state.passed || excused {
				continue
			}
			exits = true
			handedUp := !query.errorReturnsNeedWait &&
				o.handsUpError(fn, state.block, state.unchecked)
			if !state.covered && !handedUp && !query.anyPath {
				return coverage{unwaited: true, exits: true}, nil
			}
			continue
		}
		for branch, next := range state.block.Succs {
			following := flowState{
				block: next, passed: state.passed, covered: state.covered,
				unchecked: state.unchecked, nonNil: state.nonNil, held: state.held,
			}
			isNil, compared := o.errorNilOn(state, branch)
			switch {
			case compared && isNil:
				following.unchecked = literalError{}
				following.covered = true
			case compared:
				following.unchecked = literalError{}
				following.nonNil = nonNilError{bound: state.unchecked.bound}
			}
			work = append(work, following)
		}
	}
	if i < len(work) {
		return coverage{}, errWorklistFull
	}
	return coverage{unwaited: query.anyPath, exits: exits}, nil
}

func (o *owners) walk(fn function, state flowState, query flowQuery, always waiters) flowState {
	for _, node := range state.block.Nodes {
		if query.shutdown && state.passed && o.givesUp(node.Pos(), query.start) {
			state.covered = true
		}
		if query.start != nil && covers(node, query.start) {
			state.passed = true
		}
		if query.failNowEnds && state.passed && o.failsNow(node) {
			state.failed = true
			return state
		}
		state.unchecked = o.reassigned(node, state.unchecked)
		state.nonNil = o.carried(fn, node, state.nonNil)
		erring, waits := o.waitsIn(node, query, heldOnPath{fn.followed, state.held}, always)
		state.held = o.heldAfter(node, fn.followed, state.held)
		if !waits && erring != nil && state.passed {
			state = o.followError(fn, state, errorSite{node: node, call: erring}, query)
			continue
		}
		if !waits {
			continue
		}
		if _, deferred := node.(*ast.DeferStmt); deferred || state.passed {
			state.covered = true
		}
	}
	return state
}

const thenBranch = 0

type errorSite struct {
	node ast.Node
	call *ast.CallExpr
}

func (o *owners) followError(
	fn function, state flowState, site errorSite, query flowQuery,
) flowState {
	switch stmt := site.node.(type) {
	case *ast.ReturnStmt:
		handedUp := !query.errorReturnsNeedWait && o.hasErrorResult(fn) &&
			len(stmt.Results) > 0 && encloses(stmt.Results[len(stmt.Results)-1], site.call)
		state.covered = state.covered || handedUp
	case *ast.AssignStmt, *ast.ValueSpec:
		state.unchecked = o.boundError(site.call, assignmentIn(site.node))
	case ast.Expr:
		state.unchecked = literalError{call: site.call}
	}
	state.covered = state.covered || query.anyPath
	return state
}

func (o *owners) boundError(call *ast.CallExpr, found assignment) literalError {
	for i, target := range found.targets {
		value := found.valueOf(i)
		bound := o.index.Object(target)
		if value != nil && bound != nil && encloses(value, call) && holdsError(bound) {
			return literalError{call: call, bound: bound}
		}
	}
	return literalError{}
}

func (o *owners) reassigned(node ast.Node, unchecked literalError) literalError {
	if unchecked.bound == nil {
		return unchecked
	}
	found := assignmentIn(node)
	for i, target := range found.targets {
		assigned := o.index.Object(target)
		value := found.valueOf(i)
		names := value != nil && o.mentions(value, unchecked.bound)
		ident, plain := ast.Unparen(value).(*ast.Ident)
		switch {
		case assigned == nil:
		case assigned == unchecked.bound && names:
		case assigned == unchecked.bound:
			return literalError{}
		case assigned == unchecked.alias:
			unchecked.alias = nil
		case plain && o.copies(ident, unchecked) && holdsError(assigned):
			unchecked.alias = assigned
		}
	}
	return unchecked
}

func (o *owners) copies(ident *ast.Ident, unchecked literalError) bool {
	source := o.index.Object(ident)
	return source != nil && (source == unchecked.bound || source == unchecked.alias)
}

func holdsError(local *types.Var) bool {
	errorType := types.Universe.Lookup("error").Type()
	held := local.Type()
	if slice, ok := held.Underlying().(*types.Slice); ok {
		held = slice.Elem()
	}
	return types.AssignableTo(held, errorType) || types.AssignableTo(errorType, held)
}

func encloses(expr ast.Expr, call *ast.CallExpr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if node == call {
			found = true
		}
		_, isLiteral := node.(*ast.FuncLit)
		binary, isBinary := node.(*ast.BinaryExpr)
		compares := isBinary && comparison(binary.Op)
		return !found && !isLiteral && !compares
	})
	return found
}

func (o *owners) carried(fn function, node ast.Node, nonNil nonNilError) nonNilError {
	if nonNil == (nonNilError{}) {
		return nonNil
	}
	found := assignmentIn(node)
	named := o.namedError(fn)
	for i, target := range found.targets {
		assigned := o.index.Object(target)
		value := found.valueOf(i)
		uses := value != nil && assigned != nil && o.carries(value, nonNil) && holdsError(assigned)
		switch {
		case assigned == nil:
		case assigned == named && uses:
			nonNil.named = named
		case uses:
			nonNil.wrapped = assigned
		case assigned == nonNil.bound:
			nonNil.bound = nil
		case assigned == nonNil.named:
			nonNil.named = nil
		case assigned == nonNil.wrapped:
			nonNil.wrapped = nil
		}
	}
	return nonNil
}

type assignment struct {
	targets []ast.Expr
	values  []ast.Expr
}

func assignmentIn(node ast.Node) assignment {
	switch stmt := node.(type) {
	case *ast.AssignStmt:
		return assignment{targets: stmt.Lhs, values: stmt.Rhs}
	case *ast.ValueSpec:
		found := assignment{values: stmt.Values}
		for _, name := range stmt.Names {
			found.targets = append(found.targets, name)
		}
		return found
	}
	return assignment{}
}

func (a assignment) valueOf(i int) ast.Expr {
	switch {
	case len(a.values) == len(a.targets):
		return a.values[i]
	case len(a.values) == 1 && i == len(a.targets)-1:
		return a.values[0]
	}
	return nil
}

func (o *owners) carries(expr ast.Expr, nonNil nonNilError) bool {
	return o.mentions(expr, nonNil.bound) || o.mentions(expr, nonNil.named) ||
		o.mentions(expr, nonNil.wrapped)
}

func (o *owners) mentions(expr ast.Expr, local *types.Var) bool {
	if local == nil {
		return false
	}
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		switch inner := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			if comparison(inner.Op) {
				return false
			}
		}
		if ident, isIdent := node.(*ast.Ident); isIdent && o.index.Object(ident) == local {
			found = true
		}
		return !found
	})
	return found
}

func comparison(op token.Token) bool {
	return slices.Contains([]token.Token{
		token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ,
	}, op)
}

func (o *owners) errorNilOn(state flowState, branch int) (isNil, compared bool) {
	nodes := state.block.Nodes
	if state.unchecked.call == nil || len(state.block.Succs) != 2 || len(nodes) == 0 {
		return false, false
	}
	cond, ok := nodes[len(nodes)-1].(ast.Expr)
	if !ok {
		return false, false
	}
	work := []condition{{expr: cond, outcome: branch == thenBranch}}
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		item := work[i]
		switch expr := ast.Unparen(item.expr).(type) {
		case *ast.UnaryExpr:
			if expr.Op == token.NOT {
				work = append(work, condition{expr: expr.X, outcome: !item.outcome})
			}
		case *ast.BinaryExpr:
			both := expr.Op == token.LAND && item.outcome || expr.Op == token.LOR && !item.outcome
			if both {
				work = append(work,
					condition{expr: expr.X, outcome: item.outcome},
					condition{expr: expr.Y, outcome: item.outcome})
			}
			comparison := expr.Op == token.EQL || expr.Op == token.NEQ
			if comparison && o.comparedWithNil(expr, state.unchecked) {
				return (expr.Op == token.EQL) == item.outcome, true
			}
		}
	}
	assert.Ok(i == len(work), "errorNilOn: worklist reached worklist.NodesMax")
	return false, false
}

func (o *owners) returnsNonNilError(fn function, block *cfg.Block, nonNil nonNilError) bool {
	ret := finalReturn(block)
	if nonNil == (nonNilError{}) || ret == nil || !o.hasErrorResult(fn) {
		return false
	}
	if len(ret.Results) == 0 {
		named := o.namedError(fn)
		return named != nil && (named == nonNil.bound || named == nonNil.named)
	}
	last := ret.Results[len(ret.Results)-1]
	return o.carries(last, nonNil) || o.makesError(last)
}

func (o *owners) makesError(expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	name := o.calleeName(selector)
	return name == "errors.New" || name == "fmt.Errorf"
}

func (o *owners) handsUpError(fn function, block *cfg.Block, unchecked literalError) bool {
	ret := finalReturn(block)
	if unchecked.bound == nil || ret == nil || !o.hasErrorResult(fn) {
		return false
	}
	if len(ret.Results) == 0 {
		named := o.namedError(fn)
		return named != nil && (named == unchecked.bound || named == unchecked.alias)
	}
	last := ret.Results[len(ret.Results)-1]
	return o.mentions(last, unchecked.bound) || o.mentions(last, unchecked.alias)
}

func finalReturn(block *cfg.Block) *ast.ReturnStmt {
	if len(block.Nodes) == 0 {
		return nil
	}
	ret, ok := block.Nodes[len(block.Nodes)-1].(*ast.ReturnStmt)
	if !ok {
		return nil
	}
	return ret
}

func (o *owners) hasErrorResult(fn function) bool {
	if fn.typ.Results == nil {
		return false
	}
	last := fn.typ.Results.List[len(fn.typ.Results.List)-1]
	errorType := types.Universe.Lookup("error").Type()
	return types.Identical(o.pass.TypesInfo.TypeOf(last.Type), errorType)
}

func (o *owners) namedError(fn function) *types.Var {
	if !o.hasErrorResult(fn) {
		return nil
	}
	names := fn.typ.Results.List[len(fn.typ.Results.List)-1].Names
	if len(names) == 0 {
		return nil
	}
	return o.index.Object(names[len(names)-1])
}

func (o *owners) comparedWithNil(cond *ast.BinaryExpr, unchecked literalError) bool {
	isNil := func(expr ast.Expr) bool { return o.pass.TypesInfo.Types[expr].IsNil() }
	result := cond.X
	if isNil(cond.X) {
		result = cond.Y
	} else if !isNil(cond.Y) {
		return false
	}
	result = ast.Unparen(result)
	if result == unchecked.call {
		return true
	}
	compared := o.index.Object(result)
	return compared != nil && (compared == unchecked.bound || compared == unchecked.alias)
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

func (o *owners) waitsIn(
	node ast.Node, query flowQuery, onPath heldOnPath, always waiters,
) (erring *ast.CallExpr, waits bool) {
	start := query.start
	if start == nil {
		start = query.held
	}
	ast.Inspect(node, func(inner ast.Node) bool {
		switch found := inner.(type) {
		case *ast.FuncLit, *ast.GoStmt:
			return false
		case *ast.CallExpr:
			counted := !o.givesUp(found.Pos(), start)
			waits = waits || counted && o.waitCall(found, query.group, always)
			if run := o.literalRunBy(found, query.subtests, onPath); !waits && run.literal != nil {
				kind := o.waitOf(run, query, always)
				waits = kind == waitsOnEveryPath
				if kind == errorsBeforeWait && erring == nil {
					erring = found
				}
			}
		}
		return !waits
	})
	return erring, waits
}

func (o *owners) waitOf(run literalRun, query flowQuery, always waiters) literalWait {
	literal := run.literal
	if o.walking[literal] {
		return givesNoWait
	}
	held := query.held
	if query.start != nil {
		held = query.start
	}
	if held != nil && covers(literal, held) {
		return givesNoWait
	}
	o.walking[literal] = true
	defer delete(o.walking, literal)
	fn := function{decl: o.index.DeclOf(literal), typ: literal.Type, body: literal.Body}
	inner := flowQuery{group: query.group, held: held, subtests: run.within(query.subtests)}
	if unwaited, _ := o.uncovered(fn, inner, always); unwaited {
		return givesNoWait
	}
	if !o.hasErrorResult(fn) {
		return waitsOnEveryPath
	}
	inner.errorReturnsNeedWait = true
	if unwaited, _ := o.uncovered(fn, inner, always); unwaited {
		return errorsBeforeWait
	}
	return waitsOnEveryPath
}

type literalRun struct {
	literal  *ast.FuncLit
	saved    *types.Var
	subtest  *types.Var
	parallel bool
}

func (o *owners) literalRunBy(
	call *ast.CallExpr, subtests []*types.Var, onPath heldOnPath,
) literalRun {
	if literal := o.literalOnPath(call.Fun, onPath); literal != nil {
		return literalRun{literal: literal, saved: o.savedIn(call.Fun)}
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 2 || !subtestRuns[o.calleeName(selector)] {
		return literalRun{}
	}
	body := o.literalOnPath(call.Args[1], onPath)
	if body == nil {
		return literalRun{}
	}
	parallel := o.callsParallel(body)
	if parallel && !o.runsOn(selector, subtests) {
		return literalRun{}
	}
	return literalRun{
		literal:  body,
		saved:    o.savedIn(call.Args[1]),
		subtest:  o.subtestOf(body),
		parallel: parallel,
	}
}

func (run literalRun) within(subtests []*types.Var) []*types.Var {
	if run.saved != nil {
		subtests = declaredAround(subtests, run.saved)
	}
	switch {
	case run.subtest == nil:
	case run.parallel:
		subtests = []*types.Var{run.subtest}
	default:
		subtests = append(slices.Clip(subtests), run.subtest)
	}
	return subtests
}

func (o *owners) savedIn(expr ast.Expr) *types.Var {
	if _, written := ast.Unparen(expr).(*ast.FuncLit); written {
		return nil
	}
	return o.index.Object(expr)
}

func declaredAround(subtests []*types.Var, local *types.Var) []*types.Var {
	var kept []*types.Var
	for _, subtest := range subtests {
		if subtest.Parent().Contains(local.Pos()) {
			kept = append(kept, subtest)
		}
	}
	return kept
}

func (o *owners) runsOn(selector *ast.SelectorExpr, subtests []*types.Var) bool {
	decl := o.index.DeclOf(selector)
	if decl == nil {
		return false
	}
	holder := o.index.Object(selector.X)
	var aliases []*types.Var
	for range aliasHopsMax {
		if holder == nil || holder.IsField() || o.rebound(holder, decl) {
			return false
		}
		defs := o.index.Definitions(holder, decl)
		if slices.Contains(subtests, holder) {
			return len(defs) == 0 && !slices.ContainsFunc(aliases, func(alias *types.Var) bool {
				return !holder.Parent().Contains(alias.Pos())
			})
		}
		aliases = append(aliases, holder)
		if len(defs) != 1 {
			return false
		}
		holder = o.index.Object(defs[0])
	}
	return false
}

func (o *owners) subtestOf(body *ast.FuncLit) *types.Var {
	params := body.Type.Params.List
	if len(params) == 0 || len(params[0].Names) == 0 {
		return nil
	}
	return o.index.Object(params[0].Names[0])
}

var subtestRuns = map[string]bool{"(*testing.T).Run": true, "(*testing.B).Run": true}

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
	return o.parallelOn(literal.Body, o.subtestOf(literal), searched{})
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

var failNowCalls = map[string]bool{
	"(*testing.common).Fatal":   true,
	"(*testing.common).Fatalf":  true,
	"(*testing.common).FailNow": true,
	"(testing.TB).Fatal":        true,
	"(testing.TB).Fatalf":       true,
	"(testing.TB).FailNow":      true,
}

func (o *owners) failsNow(node ast.Node) bool {
	stmt, ok := node.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, isCall := stmt.X.(*ast.CallExpr)
	if !isCall {
		return false
	}
	selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	return isSelector && failNowCalls[o.calleeName(selector)]
}

func (o *owners) successExit(fn function, block *cfg.Block) bool {
	ret := finalReturn(block)
	if ret == nil || len(ret.Results) == 0 || !o.hasErrorResult(fn) {
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
