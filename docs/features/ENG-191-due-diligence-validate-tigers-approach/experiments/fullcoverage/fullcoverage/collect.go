package fullcoverage

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
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
// fixes its order.
func collectSite(pass *analysis.Pass, loop *ast.RangeStmt, after []ast.Stmt, carried *carry) *sortSite {
	targets := appendTargets(pass, loop)
	var site *sortSite
	accepted := true
	for _, target := range targets {
		call, name := firstUseSort(pass, after, target)
		arm, ok := judgeSort(pass, call, name, target)
		accepted = accepted && ok
		if site == nil && call != nil {
			site = &sortSite{pos: loop.Pos(), arm: arm, sort: name}
		}
	}
	if site == nil {
		return nil
	}
	shaped := collectShape(pass, loop, carried)
	if shaped == nil {
		site.arm = "collect-body"
		return site
	}
	for _, target := range targets {
		accepted = accepted && shaped[target]
	}
	site.accepted = accepted && len(shaped) == len(targets)
	return site
}

// appendTargets returns, in source order, every x the body updates as
// x = append(x, ...) where x is declared outside loop.
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
// earlier visit wrote. It returns nil otherwise. break and return are out:
// they make which entries are collected depend on visit order.
func collectShape(pass *analysis.Pass, loop *ast.RangeStmt, carried *carry) map[types.Object]bool {
	targets := map[types.Object]bool{}
	work := append([]ast.Stmt{}, loop.Body.List...)
	for i := 0; i < len(work); i++ {
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
	if len(targets) == 0 {
		return nil
	}
	return targets
}

// appendTarget returns x when stmt is `x = append(x, e)` with x declared
// outside loop and e pure, free of x, and free of carried state.
func appendTarget(pass *analysis.Pass, loop *ast.RangeStmt, stmt *ast.AssignStmt, carried *carry) types.Object {
	if stmt.Tok != token.ASSIGN || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return nil
	}
	lhs, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok || declaredWithin(pass, loop, lhs) {
		return nil
	}
	call, ok := stmt.Rhs[0].(*ast.CallExpr)
	if !ok || !isBuiltin(pass, call.Fun, "append") || len(call.Args) != 2 || call.Ellipsis.IsValid() {
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

// firstUseSort finds the first statement after the loop that mentions
// target. It returns that call and its name when the statement is a call to
// a function of package sort, or a sort function of package slices, whose
// first argument mentions target; otherwise nil.
func firstUseSort(pass *analysis.Pass, after []ast.Stmt, target types.Object) (*ast.CallExpr, string) {
	for _, stmt := range after {
		if !mentions(pass, stmt, target) {
			continue
		}
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			return nil, ""
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || !mentions(pass, call.Args[0], target) {
			return nil, ""
		}
		fn := calledFunc(pass, call.Fun)
		if fn == nil || fn.Pkg() == nil {
			return nil, ""
		}
		path := fn.Pkg().Path()
		if path == "sort" || (path == "slices" && strings.HasPrefix(fn.Name(), "Sort")) {
			return call, path + "." + fn.Name()
		}
		return nil, ""
	}
	return nil, ""
}

// judgeSort names the arm a sort call belongs to and reports whether it
// fixes target's order on its own.
func judgeSort(pass *analysis.Pass, call *ast.CallExpr, name string, target types.Object) (string, bool) {
	if call == nil {
		return "", false
	}
	direct := identDecl(pass, call.Args[0]) == target
	elem := sliceElem(target)
	switch {
	case orderedSorts[name]:
		return "collect-ordered", direct && len(call.Args) == 1 && orderedLeaf(elem)
	case comparatorSorts[name]:
		return "collect-comparator", direct && len(call.Args) == 2 && totalComparator(pass, call.Args[1], elem)
	}
	return "collect-other", false
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
