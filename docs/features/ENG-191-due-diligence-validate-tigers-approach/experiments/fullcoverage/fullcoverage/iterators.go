package fullcoverage

import (
	"go/ast"
	"go/types"

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
	case isFunc(pass, call.Fun, "maps", "Keys"):
		return m.Key(), true
	case isFunc(pass, call.Fun, "maps", "Values"):
		return m.Elem(), true
	case isFunc(pass, call.Fun, "maps", "All"):
		return nil, true
	}
	return nil, false
}

// checkIterators fires on every maps.Keys, maps.Values or maps.All over a
// map whose result reaches anything but a sort that fixes its order, a map
// (maps.Collect, maps.Insert), or a range statement, which checkRange
// judges as a map range.
func checkIterators(pass *analysis.Pass, file *ast.File) {
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, node)
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		elem, ok := mapIterator(pass, call)
		if !ok || iteratorFixed(pass, parent, call, elem) {
			return true
		}
		pass.Report(analysis.Diagnostic{Pos: call.Pos(), Category: "TS-T02", Message: msgIterator})
		return true
	})
}

// iteratorFixed reports whether call's only consumer, parent, fixes or
// discards its order, and records the sort sites it judges.
func iteratorFixed(pass *analysis.Pass, parent ast.Node, call *ast.CallExpr, elem types.Type) bool {
	switch p := parent.(type) {
	case *ast.RangeStmt:
		return p.X == call
	case *ast.CallExpr:
		switch {
		case isFunc(pass, p.Fun, "maps", "Collect"):
			return len(p.Args) == 1 && p.Args[0] == call
		case isFunc(pass, p.Fun, "maps", "Insert"):
			return len(p.Args) == 2 && p.Args[1] == call
		case isFunc(pass, p.Fun, "slices", "Sorted"):
			site := sortSite{pos: p.Pos(), arm: "iter-sorted", sort: "slices.Sorted", accepted: orderedLeaf(elem)}
			site.report(pass)
			return site.accepted
		case isFunc(pass, p.Fun, "slices", "SortedFunc"), isFunc(pass, p.Fun, "slices", "SortedStableFunc"):
			site := sortSite{pos: p.Pos(), arm: "iter-comparator", sort: "slices." + calledFunc(pass, p.Fun).Name()}
			site.accepted = len(p.Args) == 2 && p.Args[0] == call && totalComparator(pass, p.Args[1], elem)
			site.report(pass)
			return site.accepted
		}
	}
	return false
}
