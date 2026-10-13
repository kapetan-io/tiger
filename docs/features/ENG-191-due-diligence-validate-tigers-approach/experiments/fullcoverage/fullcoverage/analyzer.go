// Package fullcoverage is a prototype of TS-T02 (map iteration order never
// reaches an output) with the four changes ENG-191 call 8 weighs:
//
//  1. Collect-then-sort: a map loop that only appends into a local slice
//     passes when the slice's first use after the loop is sort.Strings,
//     sort.Ints or slices.Sort on string or integer elements.
//  2. Full coverage: slices.SortFunc and slices.SortStableFunc after such a
//     loop, and slices.SortedFunc over maps.Keys or maps.Values, pass when
//     the comparator compares every leaf of the element (coverage.go).
//  3. Every maps.Keys, maps.Values and maps.All result is checked; one that
//     reaches anything but a sort that fixes its order, a map, or a range is
//     a finding (iterators.go).
//  4. No read in an allowlisted loop body may see state an earlier visit
//     wrote, so a counter can no longer pick which entries are handled.
//
// The base is tiger's internal/analyzers/maporder: a range over a map is a
// finding unless its body matches a closed allowlist of order-insensitive
// shapes (allowlist.go).
package fullcoverage

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const (
	msgSortedKeys = " — range over the sorted keys instead: for _, k := range " +
		"slices.Sorted(maps.Keys(m))"
	msgAppend = "TS-T02: this loop appends to a slice while ranging over a map, and Go " +
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
		"and Go visits map entries in a different order every run, so which entries it " +
		"handles varies" + msgSortedKeys
	msgSortTies = "TS-T02: this loop collects map entries and sorts them, but the sort can " +
		"find two different entries equal, and equal entries keep the map's order, which " +
		"changes every run — sort with slices.SortFunc and a comparator that compares every " +
		"field of the element with cmp.Compare, or range over the sorted keys: " +
		"for _, k := range slices.Sorted(maps.Keys(m))"
	msgGeneric = "TS-T02: this loop's body may depend on the order of map entries, which Go " +
		"changes every run" + msgSortedKeys
)

// Analyzer enforces TS-T02 with the call 8 changes.
var Analyzer = &analysis.Analyzer{
	Name:      "fullcoverage",
	Doc:       "TS-T02: map iteration order never reaches an output (call 8 prototype).",
	Run:       run,
	FactTypes: []analysis.Fact{new(totalFact)},
}

// reportSites makes the analyzer also report every map-derived sort it
// judges, accepted or not, so the corpus scan can count both.
var reportSites bool

func init() {
	Analyzer.Flags.BoolVar(&reportSites, "sites", false,
		"also report every map-derived sort site, accepted or fired, with category \"site\"")
}

// sortSite is one place where a map's order flows into a sort.
type sortSite struct {
	pos      token.Pos
	arm      string
	sort     string
	accepted bool
}

func (s sortSite) report(pass *analysis.Pass) {
	if !reportSites {
		return
	}
	verdict := "fired"
	if s.accepted {
		verdict = "accepted"
	}
	pass.Report(analysis.Diagnostic{
		Pos:      s.pos,
		Category: "site",
		Message:  fmt.Sprintf("site arm=%s verdict=%s sort=%s", s.arm, verdict, s.sort),
	})
}

func run(pass *analysis.Pass) (any, error) {
	exportTotalFacts(pass)
	for _, file := range pass.Files {
		checkIterators(pass, file)
		after := statementsAfter(file)
		ast.Inspect(file, func(node ast.Node) bool {
			loop, ok := node.(*ast.RangeStmt)
			if ok {
				checkRange(pass, loop, after[loop])
			}
			return true
		})
	}
	return nil, nil
}

// statementsAfter maps every range statement to the statements that follow
// it in its enclosing statement list.
func statementsAfter(file *ast.File) map[*ast.RangeStmt][]ast.Stmt {
	after := map[*ast.RangeStmt][]ast.Stmt{}
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
			if loop, ok := stmt.(*ast.RangeStmt); ok {
				after[loop] = list[i+1:]
			}
		}
		return true
	})
	return after
}

// checkRange fires TS-T02 when loop ranges over a map and its body is
// neither allowlisted nor a collect-then-sort the sort makes order-free.
func checkRange(pass *analysis.Pass, loop *ast.RangeStmt, after []ast.Stmt) {
	if !isMapRange(pass, loop) {
		return
	}
	carried := carriedState(pass, loop)
	if bodyAllowed(pass, loop, carried) {
		return
	}
	site := collectSite(pass, loop, after, carried)
	if site != nil {
		site.report(pass)
		if site.accepted {
			return
		}
	}
	pass.Report(analysis.Diagnostic{
		Pos:      loop.Pos(),
		Category: "TS-T02",
		Message:  classifyMessage(pass, loop, site),
	})
}

// isMapRange reports whether loop ranges over a map (through any named type
// whose underlying type is a map) or over maps.Keys, maps.Values or maps.All
// of one.
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

// classifyMessage picks the finding message that best names what loop's
// body does that lets order escape.
func classifyMessage(pass *analysis.Pass, loop *ast.RangeStmt, site *sortSite) string {
	if site != nil && site.arm != "collect-body" {
		return msgSortTies
	}
	if bodyAllowed(pass, loop, nil) || (site != nil && collectShape(pass, loop, nil) != nil) {
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

// isPlainCall reports whether call invokes anything other than the
// allowlisted builtins len, cap, min, max, delete, and append.
func isPlainCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	for _, name := range []string{"len", "cap", "min", "max", "delete", "append"} {
		if isBuiltin(pass, call.Fun, name) {
			return false
		}
	}
	return true
}

// isStringType reports whether expr has string type.
func isStringType(pass *analysis.Pass, expr ast.Expr) bool {
	exprType := pass.TypesInfo.TypeOf(expr)
	if exprType == nil {
		return false
	}
	basic, ok := exprType.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// isBuiltin reports whether fun names the predeclared builtin name.
func isBuiltin(pass *analysis.Pass, fun ast.Expr, name string) bool {
	ident, ok := ast.Unparen(fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok && builtin.Name() == name
}

// isFunc reports whether fun names the package-level function pkg.name.
func isFunc(pass *analysis.Pass, fun ast.Expr, pkg, name string) bool {
	fn := calledFunc(pass, fun)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == pkg && fn.Name() == name
}

func calledFunc(pass *analysis.Pass, fun ast.Expr) *types.Func {
	switch f := ast.Unparen(fun).(type) {
	case *ast.IndexExpr: // explicit instantiation
		return calledFunc(pass, f.X)
	case *ast.IndexListExpr:
		return calledFunc(pass, f.X)
	case *ast.SelectorExpr:
		fn, _ := pass.TypesInfo.Uses[f.Sel].(*types.Func)
		return fn
	case *ast.Ident:
		fn, _ := pass.TypesInfo.Uses[f].(*types.Func)
		return fn
	}
	return nil
}

// orderedLeaf reports whether two values of t that compare equal are
// indistinguishable: integers and strings. Floats are left out because -0
// and +0 compare equal and print differently.
func orderedLeaf(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&(types.IsInteger|types.IsString) != 0
}
