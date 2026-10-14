package maporder

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

const msgIterator = "TS-T02: this iterator yields map entries in a different order every run, " +
	"and nothing here fixes the order — pass it straight to slices.Sorted, or to " +
	"slices.SortedFunc with a comparator that compares every field of the element with cmp.Compare"

// mapIterator returns the element type a maps.Keys or maps.Values call over
// a map yields (nil for maps.All), and whether call is such an iterator.
func mapIterator(pass *analysis.Pass, call *ast.CallExpr) (types.Type, bool) {
	if len(call.Args) != 1 {
		return nil, false
	}
	argType := pass.TypesInfo.TypeOf(call.Args[0])
	if argType == nil {
		return nil, false
	}
	m, ok := argType.Underlying().(*types.Map)
	if !ok {
		return nil, false
	}
	switch {
	case isFunc(pass, call.Fun, "maps.Keys"):
		return m.Key(), true
	case isFunc(pass, call.Fun, "maps.Values"):
		return m.Elem(), true
	case isFunc(pass, call.Fun, "maps.All"):
		return nil, true
	}
	return nil, false
}

// checkIterators fires on every maps.Keys, maps.Values or maps.All over a
// map whose result reaches anything but a sort that fixes its order, a map
// (maps.Collect, maps.Insert), or a range statement, which checkRange
// judges as a map range.
func checkIterators(pass *analysis.Pass, file *ast.File, facts fileFacts) {
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, node)
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		// Parentheses change nothing about who consumes the iterator.
		var use ast.Expr = call
		parent := ast.Node(nil)
		for _, ancestor := range slices.Backward(stack[:len(stack)-1]) {
			parent = ancestor
			paren, ok := parent.(*ast.ParenExpr)
			if !ok {
				break
			}
			use = paren
		}
		elem, ok := mapIterator(pass, call)
		if !ok || iteratorFixed(pass, parent, use, elem) ||
			collectedThenSorted(pass, stack, use, facts) {
			return true
		}
		pass.Report(analysis.Diagnostic{Pos: call.Pos(), Category: "TS-T02", Message: msgIterator})
		return true
	})
}

// iteratorFixed reports whether call's only consumer, parent, fixes or
// discards its order, and records the sort sites it judges.
func iteratorFixed(pass *analysis.Pass, parent ast.Node, call ast.Expr, elem types.Type) bool {
	switch p := parent.(type) {
	case *ast.RangeStmt:
		return p.X == call
	case *ast.CallExpr:
		switch {
		case isFunc(pass, p.Fun, "maps.Collect"):
			return len(p.Args) == 1 && p.Args[0] == call
		case isFunc(pass, p.Fun, "maps.Insert"):
			return len(p.Args) == 2 && p.Args[1] == call
		case isFunc(pass, p.Fun, "slices.Sorted"):
			return orderedLeaf(elem)
		case isFunc(pass, p.Fun, "slices.SortedFunc"),
			isFunc(pass, p.Fun, "slices.SortedStableFunc"):
			return len(p.Args) == 2 && p.Args[0] == call && totalComparator(pass, p.Args[1], elem)
		}
	}
	return false
}

// collectedThenSorted reports whether use, the iterator at the top of
// stack, is collected by slices.Collect or slices.AppendSeq into a local
// slice whose first use afterward is a sort that fixes its order.
func collectedThenSorted(
	pass *analysis.Pass, stack []ast.Node, use ast.Expr, facts fileFacts,
) bool {
	var collect *ast.CallExpr
	var stmt ast.Stmt
	if depth := slices.Index(stack, ast.Node(use)); depth >= 1 {
		collect, _ = stack[depth-1].(*ast.CallExpr)
		if depth >= 2 {
			stmt, _ = stack[depth-2].(ast.Stmt)
		}
	}
	assign, ok := stmt.(*ast.AssignStmt)
	if collect == nil || !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 ||
		assign.Rhs[0] != ast.Expr(collect) {
		return false
	}
	target := identDecl(pass, assign.Lhs[0])
	switch {
	case isFunc(pass, collect.Fun, "slices.Collect"):
		if len(collect.Args) != 1 {
			return false
		}
	case isFunc(pass, collect.Fun, "slices.AppendSeq"):
		if len(collect.Args) != 2 || collect.Args[1] != use {
			return false
		}
		// A fresh slice from make, or the target extended in place.
		fresh := false
		if made, ok := ast.Unparen(collect.Args[0]).(*ast.CallExpr); ok {
			fresh = isBuiltin(pass, made.Fun, "make")
		}
		if !fresh && identDecl(pass, collect.Args[0]) != target {
			return false
		}
	default:
		return false
	}
	if target == nil || target.Parent() == pass.Pkg.Scope() {
		return false
	}
	sorted := firstUseSort(pass, facts.after[assign], target)
	if first, ok := facts.shared[target]; ok && sorted != nil && first < sorted.call.Pos() {
		return false
	}
	return sortFixesOrder(pass, sorted, target)
}
