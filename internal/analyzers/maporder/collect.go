package maporder

import (
	"go/ast"
	"go/token"
	"go/types"
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
	pass *analysis.Pass, loop *ast.RangeStmt, after []ast.Stmt, carried *carry,
	shared map[types.Object]token.Pos,
) *sortSite {
	targets := appendTargets(pass, loop)
	var site *sortSite
	accepted := true
	for _, target := range targets {
		sorted := firstUseSort(pass, after, target)
		if target.Parent() == pass.Pkg.Scope() {
			return nil
		}
		if first, ok := shared[target]; ok && sorted != nil && first < sorted.call.Pos() {
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
	return targets
}

// collectShape reports the append targets when loop's body is allowlisted
// statements plus x = append(x, e) with e pure, and nothing reads state an
// earlier visit wrote. It returns nil otherwise. A break or return is out,
// because it makes which entries are collected depend on visit order.
func collectShape(pass *analysis.Pass, loop *ast.RangeStmt, carried *carry) map[types.Object]bool {
	targets := map[types.Object]bool{}
	work := append([]ast.Stmt{}, loop.Body.List...)
	i := 0
	for ; i < len(work) && i < worklist.NodesMax; i++ {
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

// firstUseSort finds the first statement after the loop that mentions
// target. It returns the call when the statement is a call to a function of
// package sort, or a sort function of package slices, whose first argument
// mentions target; otherwise nil.
func firstUseSort(pass *analysis.Pass, after []ast.Stmt, target types.Object) *sortCall {
	for _, stmt := range after {
		if !mentions(pass, stmt, target) {
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
// possible: its address is taken, a function literal captures it, or it is
// sliced, so the slice shares its backing array. A read through any of these
// before the sort sees the map's order, and the first-use check looks only at
// mentions of the name.
func sharedVars(pass *analysis.Pass, file *ast.File) map[types.Object]token.Pos {
	shared := map[types.Object]token.Pos{}
	mark := func(root ast.Node, keep func(types.Object) bool) {
		ast.Inspect(root, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			obj, ok := pass.TypesInfo.ObjectOf(ident).(*types.Var)
			if !ok || !keep(obj) {
				return true
			}
			if first, seen := shared[obj]; !seen || ident.Pos() < first {
				shared[obj] = ident.Pos()
			}
			return true
		})
	}
	always := func(types.Object) bool { return true }
	ast.Inspect(file, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.UnaryExpr:
			if expr.Op == token.AND {
				mark(expr.X, always)
			}
		case *ast.SliceExpr:
			mark(expr.X, always)
		case *ast.FuncLit:
			mark(expr.Body, func(obj types.Object) bool {
				return obj.Pos() < expr.Pos() || obj.Pos() >= expr.End()
			})
		}
		return true
	})
	return shared
}
