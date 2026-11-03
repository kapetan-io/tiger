// Package maporder enforces TS-T02: map iteration order never reaches an
// output.
//
// A range over a map is a finding unless its body matches a closed
// allowlist of order-insensitive shapes (allowlist.go), with these
// additions recorded in ADR-0021:
//
//  1. Collect-then-sort: a map loop that only appends into a function-local
//     slice, or fills it by position (s[i] = e; i++), passes when the
//     slice's first use after the loop is a sort that fixes its order:
//     sort.Strings, sort.Ints, slices.Sort, or sort.Slice comparing the
//     slice's own elements. A slice another name can read before the sort
//     (address taken, captured, sliced, copied) never passes.
//  2. Full coverage: slices.SortFunc and slices.SortStableFunc after such a
//     loop, and slices.SortedFunc over maps.Keys or maps.Values, pass when
//     the comparator compares every leaf of the element (coverage.go).
//  3. Every maps.Keys, maps.Values and maps.All result is checked; one that
//     reaches anything but a sort that fixes its order, a map, a range, or
//     slices.Collect or slices.AppendSeq into a local that is then sorted, is
//     a finding (iterators.go).
//  4. No read in an allowlisted loop body may see state an earlier visit
//     wrote, so a counter cannot pick which entries are handled.
//  5. A body that can break or return must change nothing that outlives one
//     visit, and each exit must carry every constant outcome the body can
//     set, because which entries a break skips depends on visit order.
//
// The allowlist is closed and versioned with this analyzer: extending it to
// a new provably-safe shape is an analyzer change with its own corpus case,
// never a configuration knob.
package maporder

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

const (
	msgSortedKeys = " — range over slices.Sorted(maps.Keys(m)) instead"
	msgAppend     = "TS-T02: this loop appends to a slice while ranging over a map, and Go " +
		"visits map entries in a different order every run, so the slice's order varies" +
		msgSortedKeys
	msgCall = "TS-T02: this loop calls a function on each map entry, and Go visits map " +
		"entries in a different order every run, so the calls happen in varying order" +
		msgSortedKeys
	msgStringBuild = "TS-T02: this loop builds a string while ranging over a map, and Go " +
		"visits map entries in a different order every run, so the string varies" +
		msgSortedKeys
	msgChannelSend = "TS-T02: this loop sends on a channel while ranging over a map, and Go " +
		"visits map entries in a different order every run, so the receiver's order varies" +
		msgSortedKeys
	msgCarried = "TS-T02: this loop reads state it changed while visiting earlier map entries, " +
		"and Go visits entries in a different order every run, so what it " +
		"handles varies" + msgSortedKeys
	msgEarlyExit = "TS-T02: this loop changes state and can stop early, and Go visits map " +
		"entries in a different order every run, so which entries it handled varies" + msgSortedKeys
	msgSortTies = "TS-T02: this sort can find two different map entries equal, and equal entries " +
		"keep the map's order — compare every field with cmp.Compare, or use " +
		"slices.Sorted(maps.Keys(m)), then slices.SortStableFunc"
	msgGeneric = "TS-T02: this loop's body may depend on the order of map entries, which Go " +
		"changes every run" + msgSortedKeys
)

var Analyzer = &analysis.Analyzer{
	Name:      "maporder",
	Doc:       "TS-T02: map iteration order never reaches an output.",
	Run:       run,
	FactTypes: []analysis.Fact{new(totalFact)},
}

type sortSite struct {
	bodyImpure bool
	accepted   bool
}

func run(pass *analysis.Pass) (any, error) {
	exportTotalFacts(pass)
	for _, file := range pass.Files {
		facts := newFileFacts(pass, file)
		checkIterators(pass, file, facts)
		ast.Inspect(file, func(node ast.Node) bool {
			loop, ok := node.(*ast.RangeStmt)
			if ok {
				checkRange(pass, loop, facts)
			}
			return true
		})
	}
	return nil, nil
}

func statementsAfter(file *ast.File) map[ast.Stmt][]ast.Stmt {
	after := map[ast.Stmt][]ast.Stmt{}
	ast.Inspect(file, func(node ast.Node) bool {
		var list []ast.Stmt
		switch block := node.(type) {
		case *ast.BlockStmt:
			list = block.List
		case *ast.CaseClause:
			list = block.Body
		case *ast.CommClause:
			list = block.Body
		}
		for i, stmt := range list {
			after[unlabel(stmt)] = list[i+1:]
		}
		return true
	})
	return after
}

func unlabel(stmt ast.Stmt) ast.Stmt {
	for range maxPathDepth {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = labeled.Stmt
	}
	return stmt
}

type span struct{ pos, end token.Pos }

type fileFacts struct {
	after   map[ast.Stmt][]ast.Stmt
	shared  map[types.Object]token.Pos
	loops   map[ast.Stmt]span
	results map[types.Object]bool
}

func newFileFacts(pass *analysis.Pass, file *ast.File) fileFacts {
	facts := fileFacts{
		after:   statementsAfter(file),
		shared:  sharedVars(pass, file),
		loops:   map[ast.Stmt]span{},
		results: map[types.Object]bool{},
	}
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		switch node := node.(type) {
		case *ast.FuncDecl:
			facts.addResults(pass, node.Type)
		case *ast.FuncLit:
			facts.addResults(pass, node.Type)
		case *ast.RangeStmt:
			facts.noteLoop(stack, node)
		case *ast.AssignStmt:
			facts.noteLoop(stack, node)
		}
		stack = append(stack, node)
		return true
	})
	return facts
}

func (facts fileFacts) noteLoop(stack []ast.Node, stmt ast.Stmt) {
	if outer, ok := outermostLoop(stack); ok {
		facts.loops[stmt] = outer
	}
}

func (facts fileFacts) addResults(pass *analysis.Pass, sig *ast.FuncType) {
	if sig.Results == nil {
		return
	}
	for _, field := range sig.Results.List {
		for _, name := range field.Names {
			if obj := pass.TypesInfo.ObjectOf(name); obj != nil {
				facts.results[obj] = true
			}
		}
	}
}

func outermostLoop(stack []ast.Node) (span, bool) {
	var outer span
	found := false
	for _, node := range slices.Backward(stack) {
		switch node := node.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return outer, found
		case *ast.ForStmt, *ast.RangeStmt:
			outer, found = span{node.Pos(), node.End()}, true
		}
	}
	return outer, found
}

func (facts fileFacts) sortAfter(
	pass *analysis.Pass, at ast.Stmt, target types.Object,
) (sorted *sortCall, aliased bool) {
	mode := plainLocal
	if facts.results[target] {
		mode = namedResult
	}
	sorted = firstUseSort(pass, facts.after[at], target, mode)
	if target.Parent() == pass.Pkg.Scope() {
		return sorted, true
	}
	first, shared := facts.shared[target]
	if !shared || sorted == nil {
		return sorted, false
	}
	limit := sorted.call.Pos()
	if loop, ok := facts.loops[at]; ok && target.Pos() < loop.pos {
		limit = loop.end
	}
	return sorted, first < limit
}

func checkRange(pass *analysis.Pass, loop *ast.RangeStmt, facts fileFacts) {
	if !isMapRange(pass, loop) {
		return
	}
	carried := carriedState(pass, loop)
	if bodyAllowed(pass, loop, carried) {
		return
	}
	site := collectSite(pass, loop, carried, facts)
	if site != nil && site.accepted {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-T02",
		Message:  classifyMessage(pass, loop, site),
	})
}

func isMapRange(pass *analysis.Pass, loop *ast.RangeStmt) bool {
	if call, ok := ast.Unparen(loop.X).(*ast.CallExpr); ok {
		if _, ok := mapIterator(pass, call); ok {
			return true
		}
	}
	rangeType := pass.TypesInfo.TypeOf(loop.X)
	if rangeType == nil {
		return false
	}
	_, mapped := rangeType.Underlying().(*types.Map)
	return mapped
}

func classifyMessage(pass *analysis.Pass, loop *ast.RangeStmt, site *sortSite) string {
	if site != nil && !site.bodyImpure {
		return msgSortTies
	}
	if allowed, unstable := walkBody(pass, loop, nil); allowed {
		if unstable {
			return msgEarlyExit
		}
		return msgCarried
	}
	if site != nil && collectShape(pass, loop, nil) != nil {
		return msgCarried
	}
	found := ""
	ast.Inspect(loop.Body, func(node ast.Node) bool {
		if found != "" {
			return false
		}
		switch signal := node.(type) {
		case *ast.SendStmt:
			found = msgChannelSend
		case *ast.BinaryExpr:
			if signal.Op == token.ADD && isStringType(pass, signal) {
				found = msgStringBuild
			}
		case *ast.CallExpr:
			if isBuiltin(pass, signal.Fun, "append") {
				found = msgAppend
			} else if isPlainCall(pass, signal) {
				found = msgCall
			}
		}
		return found == ""
	})
	if found == "" {
		return msgGeneric
	}
	return found
}

func isPlainCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	for _, name := range []string{"len", "cap", "min", "max", "delete", "append"} {
		if isBuiltin(pass, call.Fun, name) {
			return false
		}
	}
	return true
}

func isStringType(pass *analysis.Pass, expr ast.Expr) bool {
	exprType := pass.TypesInfo.TypeOf(expr)
	if exprType == nil {
		return false
	}
	basic, ok := exprType.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

func isBuiltin(pass *analysis.Pass, fun ast.Expr, name string) bool {
	ident, ok := ast.Unparen(fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok && builtin.Name() == name
}

func isFunc(pass *analysis.Pass, fun ast.Expr, qualified string) bool {
	fn := calledFunc(pass, fun)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path()+"."+fn.Name() == qualified
}

const maxInstantiation = 8

func calledFunc(pass *analysis.Pass, fun ast.Expr) *types.Func {
	for range maxInstantiation {
		switch f := ast.Unparen(fun).(type) {
		case *ast.IndexExpr:
			fun = f.X
		case *ast.IndexListExpr:
			fun = f.X
		case *ast.SelectorExpr:
			fn, ok := pass.TypesInfo.Uses[f.Sel].(*types.Func)
			if !ok {
				return nil
			}
			return fn
		case *ast.Ident:
			fn, ok := pass.TypesInfo.Uses[f].(*types.Func)
			if !ok {
				return nil
			}
			return fn
		default:
			return nil
		}
	}
	return nil
}

func orderedLeaf(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&(types.IsInteger|types.IsString) != 0
}
