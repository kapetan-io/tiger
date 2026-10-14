package maporder

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/worklist"
)

// orderedSorts sort a slice by the values themselves, so the result is
// fixed by the multiset of values when equal values can't be told apart.
var orderedSorts = map[string]bool{
	"sort.Strings":  true,
	"sort.Ints":     true,
	"sort.Float64s": true, // judged, never accepted: floats are not orderedLeaf
	"slices.Sort":   true,
}

// indexSorts order a slice by a less function over indexes.
var indexSorts = map[string]bool{
	"sort.Slice":       true,
	"sort.SliceStable": true,
}

// comparatorSorts are accepted only with a full-coverage comparator.
var comparatorSorts = map[string]bool{
	"slices.SortFunc":       true,
	"slices.SortStableFunc": true,
}

// collectSite judges a map loop that appends into slices declared outside
// it. It returns nil when no append target's first use after the loop is a
// sort. Otherwise the site is accepted only when the body is a pure
// collection (collectShape) and every target's first use is a sort that
// fixes its order. A target that something else can read under another name
// (sharedVars) before the sort makes the sort no proof, so such a loop is no collect site.
func collectSite(
	pass *analysis.Pass, loop *ast.RangeStmt, carried *carry, facts fileFacts,
) *sortSite {
	targets := appendTargets(pass, loop)
	var site *sortSite
	accepted := true
	for _, target := range targets {
		sorted, aliased := facts.sortAfter(pass, loop, target)
		if aliased {
			return nil
		}
		accepted = accepted && sortFixesOrder(pass, sorted, target)
		if site == nil && sorted != nil {
			site = &sortSite{}
		}
	}
	if site == nil {
		return nil
	}
	shaped := collectShape(pass, loop, carried)
	if shaped == nil {
		site.bodyImpure = true
		return site
	}
	for _, target := range targets {
		accepted = accepted && shaped[target]
	}
	site.accepted = accepted && len(shaped) == len(targets)
	return site
}

// appendTargets returns, in source order, every x the body updates by
// appending to x itself, where x is declared outside loop.
func appendTargets(pass *analysis.Pass, loop *ast.RangeStmt) []types.Object {
	var targets []types.Object
	seen := map[types.Object]bool{}
	ast.Inspect(loop.Body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || declaredWithin(pass, loop, lhs) {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || !isBuiltin(pass, call.Fun, "append") || len(call.Args) == 0 {
			return true
		}
		target := pass.TypesInfo.ObjectOf(lhs)
		if target != nil && identDecl(pass, call.Args[0]) == target && !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
		return true
	})
	for _, pair := range indexedPairs(pass, loop) {
		if !seen[pair.slice] {
			seen[pair.slice] = true
			targets = append(targets, pair.slice)
		}
	}
	return targets
}

// indexedPair is the pair of statements `s[i] = e` and `i++` that fill the
// slice s by position.
type indexedPair struct {
	slice, counter types.Object
	store          *ast.AssignStmt
	step           *ast.IncDecStmt
}

// indexedPairs finds the adjacent top-level statement pairs `s[i] = e; i++` in loop's
// body where s is a slice and i an integer, both declared outside loop, i
// appears nowhere else in the body, s appears only as the store's target,
// and e reads neither. Each visit then writes its own slot, so the slice
// holds the same values whatever the visit order, and only their position
// varies, exactly as with append.
func indexedPairs(pass *analysis.Pass, loop *ast.RangeStmt) []indexedPair {
	var pairs []indexedPair
	for at, stmt := range loop.Body.List {
		store, ok := stmt.(*ast.AssignStmt)
		if !ok || store.Tok != token.ASSIGN || len(store.Lhs) != 1 || len(store.Rhs) != 1 {
			continue
		}
		index, ok := ast.Unparen(store.Lhs[0]).(*ast.IndexExpr)
		if !ok {
			continue
		}
		slice, counter := identDecl(pass, index.X), identDecl(pass, index.Index)
		if slice == nil || counter == nil || sliceElem(slice) == nil || !isIntVar(counter) {
			continue
		}
		if declaredWithinObj(loop, slice) || declaredWithinObj(loop, counter) {
			continue
		}
		if !pureExpr(pass, store.Rhs[0]) ||
			readsAny(pass, store.Rhs[0], map[types.Object]bool{slice: true, counter: true}) {
			continue
		}
		step := stepAfter(pass, loop.Body.List, at, counter)
		if step == nil || countMentions(pass, loop.Body, counter) != 2 ||
			countMentions(pass, loop.Body, slice) != 1 {
			continue
		}
		pairs = append(pairs, indexedPair{slice: slice, counter: counter, store: store, step: step})
	}
	return pairs
}

// stepAfter returns the `counter++` statement directly after list[at]. Any
// statement between them could skip the step, and the next visit would then
// overwrite the slot, so which key survives would depend on visit order.
func stepAfter(pass *analysis.Pass, list []ast.Stmt, at int, counter types.Object) *ast.IncDecStmt {
	if at+1 >= len(list) {
		return nil
	}
	step, ok := list[at+1].(*ast.IncDecStmt)
	if ok && step.Tok == token.INC && identDecl(pass, step.X) == counter {
		return step
	}
	return nil
}

func isIntVar(obj types.Object) bool {
	basic, ok := obj.Type().Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}

func declaredWithinObj(loop *ast.RangeStmt, obj types.Object) bool {
	return obj.Pos() >= loop.Body.Pos() && obj.Pos() < loop.Body.End()
}

// countMentions counts the identifiers in node that refer to obj.
func countMentions(pass *analysis.Pass, node ast.Node, obj types.Object) int {
	count := 0
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && pass.TypesInfo.ObjectOf(ident) == obj {
			count++
		}
		return true
	})
	return count
}

// collectShape reports the append targets when loop's body is allowlisted
// statements plus x = append(x, e) with e pure, and nothing reads state an
// earlier visit wrote. It returns nil otherwise. A break or return is out,
// because it makes which entries are collected depend on visit order.
func collectShape(pass *analysis.Pass, loop *ast.RangeStmt, carried *carry) map[types.Object]bool {
	if carried != nil && carried.reshapes {
		return nil
	}
	targets := map[types.Object]bool{}
	work := append([]ast.Stmt{}, loop.Body.List...)
	pairs := indexedPairs(pass, loop)
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
		if pair, ok := pairOf(pairs, work[i]); ok {
			if pair.store == work[i] && clean(pass, pair.store.Rhs[0], carried) {
				targets[pair.slice] = true
			} else if pair.store == work[i] {
				return nil
			}
			continue
		}
		switch stmt := work[i].(type) {
		case *ast.AssignStmt:
			if target := appendTarget(pass, loop, stmt, carried); target != nil {
				targets[target] = true
				continue
			}
		case *ast.BranchStmt:
			if stmt.Label != nil || stmt.Tok != token.CONTINUE {
				return nil
			}
			continue
		case *ast.ReturnStmt:
			return nil
		}
		next, ok := stmtAllowed(pass, loop, work[i], carried)
		if !ok {
			return nil
		}
		work = append(work, next...)
	}
	assert.Ok(i == len(work), "collectShape: worklist reached worklist.NodesMax")
	if len(targets) == 0 {
		return nil
	}
	return targets
}

// pairOf returns the pair that stmt is the store or the step of.
func pairOf(pairs []indexedPair, stmt ast.Stmt) (indexedPair, bool) {
	for _, pair := range pairs {
		if stmt == ast.Stmt(pair.store) || stmt == ast.Stmt(pair.step) {
			return pair, true
		}
	}
	return indexedPair{}, false
}

// appendTarget returns x when stmt is `x = append(x, e)` with x declared
// outside loop and e pure, free of x, and free of carried state.
func appendTarget(
	pass *analysis.Pass, loop *ast.RangeStmt, stmt *ast.AssignStmt, carried *carry,
) types.Object {
	if stmt.Tok != token.ASSIGN || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return nil
	}
	lhs, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok || declaredWithin(pass, loop, lhs) {
		return nil
	}
	call, ok := stmt.Rhs[0].(*ast.CallExpr)
	if !ok || !isBuiltin(pass, call.Fun, "append") || len(call.Args) != 2 {
		return nil
	}
	if call.Ellipsis.IsValid() {
		return nil
	}
	target := pass.TypesInfo.ObjectOf(lhs)
	if identDecl(pass, call.Args[0]) != target {
		return nil
	}
	elem := call.Args[1]
	if !pureExpr(pass, elem) || readsAny(pass, elem, map[types.Object]bool{target: true}) {
		return nil
	}
	if !clean(pass, elem, carried) {
		return nil
	}
	return target
}

// sortCall is a call to a sort function and the qualified name it resolved to.
type sortCall struct {
	call *ast.CallExpr
	name string
}

// resultMode says whether the slice a sort fixes is one of its function's
// named results.
type resultMode bool

const (
	plainLocal  resultMode = false
	namedResult resultMode = true
)

// firstUseSort finds the first statement after the loop that mentions
// target. It returns the call when the statement is a call to a function of
// package sort, or a sort function of package slices, whose first argument
// mentions target; otherwise nil.
func firstUseSort(
	pass *analysis.Pass, after []ast.Stmt, target types.Object, results resultMode,
) *sortCall {
	for _, stmt := range after {
		if !mentions(pass, stmt, target) {
			if leavesList(stmt, results) {
				return nil
			}
			continue
		}
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			return nil
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || !mentions(pass, call.Args[0], target) {
			return nil
		}
		fn := calledFunc(pass, call.Fun)
		if fn == nil || fn.Pkg() == nil || !isSortFunc(fn) {
			return nil
		}
		return &sortCall{call: call, name: fn.Pkg().Path() + "." + fn.Name()}
	}
	return nil
}

// leavesList reports whether stmt holds a break, continue or goto outside a
// function literal. Such a jump can skip the sort for the rest of an
// enclosing loop, which then sees the slice in map order. A bare return
// counts when the target is a named result, because it hands back the named result unsorted.
func leavesList(stmt ast.Stmt, results resultMode) bool {
	found := false
	ast.Inspect(stmt, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			found = found || node.Tok != token.FALLTHROUGH
		case *ast.ReturnStmt:
			if results == namedResult && len(node.Results) == 0 {
				found = true
			}
		}
		return !found
	})
	return found
}

// isSortFunc reports whether fn is in package sort, or is a Sort* function
// of package slices.
func isSortFunc(fn *types.Func) bool {
	switch fn.Pkg().Path() {
	case "sort":
		return true
	case "slices":
		return strings.HasPrefix(fn.Name(), "Sort")
	}
	return false
}

// sortFixesOrder reports whether the sort call leaves target in an order
// that does not depend on the map's: a plain sort of strings or integers, or
// a comparator that covers every field of the element.
func sortFixesOrder(pass *analysis.Pass, sorted *sortCall, target types.Object) bool {
	if sorted == nil {
		return false
	}
	call := sorted.call
	direct := identDecl(pass, call.Args[0]) == target
	elem := sliceElem(target)
	switch {
	case orderedSorts[sorted.name]:
		return direct && len(call.Args) == 1 && orderedLeaf(elem)
	case indexSorts[sorted.name]:
		return direct && len(call.Args) == 2 && orderedLeaf(elem) &&
			compareElements(pass, call.Args[1], target)
	case comparatorSorts[sorted.name]:
		total := len(call.Args) == 2 && totalComparator(pass, call.Args[1], elem)
		return direct && total
	}
	return false
}

// mentions reports whether node refers to obj anywhere.
func mentions(pass *analysis.Pass, node ast.Node, obj types.Object) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && pass.TypesInfo.ObjectOf(ident) == obj {
			found = true
		}
		return !found
	})
	return found
}

// sliceElem returns the element type of a slice-typed object.
func sliceElem(obj types.Object) types.Type {
	s, ok := obj.Type().Underlying().(*types.Slice)
	if !ok {
		return nil
	}
	return s.Elem()
}

// sharedVars returns, for each variable in file that code other than a plain
// mention of the name can read, the first position where that becomes
// possible. A slice variable is aliased once its header is copied (assigned,
// passed, returned, stored), its address is taken, a function literal
// captures it, it is sliced, or it starts out as something other than a
// fresh slice. All of these share the backing array, so a read through the
// other name before the sort sees the map's order, and the first-use check
// looks only at mentions of the name. Using the slice as `s = append(s, e)`,
// `len(s)`, `cap(s)`, `s[i]` or a range operand leaves it unaliased.
func sharedVars(pass *analysis.Pass, file *ast.File) map[types.Object]token.Pos {
	shared := map[types.Object]token.Pos{}
	mark := func(obj types.Object, pos token.Pos) {
		if first, seen := shared[obj]; !seen || pos < first {
			shared[obj] = pos
		}
	}
	markAll := func(root ast.Node, keep func(types.Object) bool) {
		ast.Inspect(root, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			if obj, ok := pass.TypesInfo.ObjectOf(ident).(*types.Var); ok && keep(obj) {
				mark(obj, ident.Pos())
			}
			return true
		})
	}
	always := func(types.Object) bool { return true }
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, node)
		switch expr := node.(type) {
		case *ast.UnaryExpr:
			if expr.Op == token.AND {
				markAll(expr.X, always)
			}
		case *ast.SliceExpr:
			markAll(expr.X, always)
		case *ast.FuncLit:
			markAll(expr.Body, func(obj types.Object) bool {
				return obj.Pos() < expr.Pos() || obj.Pos() >= expr.End()
			})
		case *ast.AssignStmt:
			for at, lhs := range expr.Lhs {
				markStaleStart(pass, lhs, mark, startOf(len(expr.Lhs), expr.Rhs, at))
			}
		case *ast.ValueSpec:
			for at, name := range expr.Names {
				markStaleStart(pass, name, mark, startOf(len(expr.Names), expr.Values, at))
			}
		case *ast.Ident:
			obj, ok := pass.TypesInfo.ObjectOf(expr).(*types.Var)
			if !ok || pass.TypesInfo.Defs[expr] != nil || !isSliceType(obj.Type()) {
				break
			}
			if copiesHeader(pass, stack, obj) {
				mark(obj, expr.Pos())
			}
		}
		return true
	})
	return shared
}

// startOf returns the expression that gives the at-th of count variables its
// value, or nil when a single multi-value call or receive gives several of
// them theirs, or when none is given (a plain declaration).
func startOf(count int, values []ast.Expr, at int) ast.Expr {
	if len(values) == count {
		return values[at]
	}
	if len(values) == 1 {
		return values[0]
	}
	return nil
}

// markStaleStart marks the slice variable lhs when rhs is not a fresh slice
// (make, a literal, nil, or the variable appended to), because it may then
// share storage with whatever rhs came from.
func markStaleStart(
	pass *analysis.Pass, lhs ast.Expr, mark func(types.Object, token.Pos), rhs ast.Expr,
) {
	ident, ok := ast.Unparen(lhs).(*ast.Ident)
	if !ok {
		return
	}
	obj, ok := pass.TypesInfo.ObjectOf(ident).(*types.Var)
	if !ok || !isSliceType(obj.Type()) || rhs == nil {
		return
	}
	if _, tuple := pass.TypesInfo.TypeOf(rhs).(*types.Tuple); tuple {
		mark(obj, ident.Pos())
		return
	}
	switch fresh := ast.Unparen(rhs).(type) {
	case *ast.CompositeLit:
		return
	case *ast.Ident:
		if pass.TypesInfo.ObjectOf(fresh) == types.Universe.Lookup("nil") {
			return
		}
	case *ast.CallExpr:
		if isBuiltin(pass, fresh.Fun, "make") || isFunc(pass, fresh.Fun, "slices.Collect") {
			return
		}
		if len(fresh.Args) == 0 {
			break
		}
		if isFunc(pass, fresh.Fun, "slices.AppendSeq") && freshBase(pass, fresh.Args[0], obj) {
			return
		}
		if isBuiltin(pass, fresh.Fun, "append") && identDecl(pass, fresh.Args[0]) == obj {
			return
		}
	}
	mark(obj, ident.Pos())
}

// freshBase reports whether expr is a make call or the variable obj itself.
func freshBase(pass *analysis.Pass, expr ast.Expr, obj types.Object) bool {
	if made, ok := ast.Unparen(expr).(*ast.CallExpr); ok && isBuiltin(pass, made.Fun, "make") {
		return true
	}
	return identDecl(pass, expr) == obj
}

func isSliceType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

// copiesHeader reports whether the identifier at the top of stack uses the
// slice obj in a way that can hand its backing array to another name.
func copiesHeader(pass *analysis.Pass, stack []ast.Node, obj types.Object) bool {
	if len(stack) < 2 {
		return false
	}
	ident := stack[len(stack)-1]
	switch parent := stack[len(stack)-2].(type) {
	case *ast.AssignStmt:
		return !slices.ContainsFunc(parent.Lhs, func(lhs ast.Expr) bool { return lhs == ident })
	case *ast.ValueSpec:
		return !slices.ContainsFunc(parent.Names, func(name *ast.Ident) bool {
			return name == ident
		})
	case *ast.IndexExpr:
		return parent.X != ident
	case *ast.RangeStmt:
		return parent.X != ident
	case *ast.CallExpr:
		if isBuiltin(pass, parent.Fun, "len") || isBuiltin(pass, parent.Fun, "cap") {
			return false
		}
		extends := isBuiltin(pass, parent.Fun, "append") ||
			isFunc(pass, parent.Fun, "slices.AppendSeq")
		if extends && len(parent.Args) > 0 && parent.Args[0] == ident {
			return !appendsToItself(pass, stack, parent, obj)
		}
	}
	return true
}

// appendsToItself reports whether call is the right side of `obj = call`.
func appendsToItself(
	pass *analysis.Pass, stack []ast.Node, call *ast.CallExpr, obj types.Object,
) bool {
	if len(stack) < 3 {
		return false
	}
	assign, ok := stack[len(stack)-3].(*ast.AssignStmt)
	return ok && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 && assign.Rhs[0] == ast.Expr(call) &&
		identDecl(pass, assign.Lhs[0]) == obj
}

// compareElements reports whether less is func(i, j int) bool { return
// s[i] < s[j] } (or >) over the slice target itself, a total order on string
// and integer elements.
func compareElements(pass *analysis.Pass, less ast.Expr, target types.Object) bool {
	lit, ok := ast.Unparen(less).(*ast.FuncLit)
	if !ok || lit.Type.Params.NumFields() != 2 || len(lit.Body.List) != 1 {
		return false
	}
	var params []types.Object
	for _, field := range lit.Type.Params.List {
		for _, name := range field.Names {
			params = append(params, pass.TypesInfo.ObjectOf(name))
		}
	}
	ret, ok := lit.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 || len(params) != 2 {
		return false
	}
	cmp, ok := ast.Unparen(ret.Results[0]).(*ast.BinaryExpr)
	if !ok || cmp.Op == token.EQL {
		return false
	}
	if cmp.Op != token.LSS && cmp.Op != token.GTR {
		return false
	}
	at := func(expr ast.Expr) types.Object {
		index, ok := ast.Unparen(expr).(*ast.IndexExpr)
		if !ok || identDecl(pass, index.X) != target {
			return nil
		}
		return identDecl(pass, index.Index)
	}
	left, right := at(cmp.X), at(cmp.Y)
	if left == nil || right == nil || left == right {
		return false
	}
	if left == params[0] {
		return right == params[1]
	}
	return left == params[1] && right == params[0]
}
