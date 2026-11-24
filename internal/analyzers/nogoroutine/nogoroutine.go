// Package nogoroutine enforces TS-C02, TS-C03 and TS-C09: every goroutine
// starts through a group that waits for it, packages that start goroutines
// check for leaks in their tests, and loops don't spawn per item.
package nogoroutine

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "nogoroutine",
	Doc:  "TS-C02, TS-C03 and TS-C09: every goroutine starts through a group that waits for it.",
	Run:  run,
}

const (
	msgUnowned = "TS-C02: this go statement starts a goroutine nobody owns — start it with " +
		"wg.Go or errgroup.Go and Wait for the group"
	msgNoWaitFmt = "TS-C02: this %s starts a goroutine, but no Wait on the group is reached on " +
		"the success path — Wait before returning, or from an exported method like " +
		"Shutdown"
	msgReactive = "TS-C09: this loop starts a goroutine per item, so the goroutines react to " +
		"each item instead of working at their own pace — append the items to a bounded " +
		"queue and drain it from one supervisor goroutine"
	msgNoLeakCheck = "TS-C03: this package starts goroutines but its tests have no goleak " +
		"check — add a TestMain that calls goleak.VerifyTestMain(m)"
)

func run(pass *analysis.Pass) (any, error) {
	owners := newOwners(pass)
	starts := []ast.Node{}
	for _, file := range pass.Files {
		var ancestors []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				ancestors = ancestors[:len(ancestors)-1]
				return false
			}
			switch stmt := node.(type) {
			case *ast.GoStmt:
				check(pass, stmt, ancestors)
				starts = append(starts, stmt)
			case *ast.SelectorExpr:
				if owners.checkStart(stmt, ancestors) {
					starts = append(starts, stmt)
				}
			}
			ancestors = append(ancestors, node)
			return true
		})
	}
	checkHarness(pass, starts)
	return nil, nil
}

func check(pass *analysis.Pass, spawn *ast.GoStmt, ancestors []ast.Node) {
	if insideLoop(ancestors) {
		reportReactive(pass, spawn.Pos())
		return
	}
	pass.Report(analysis.Diagnostic{Pos: spawn.Pos(), Category: "TS-C02", Message: msgUnowned})
}

func reportReactive(pass *analysis.Pass, pos token.Pos) {
	if strings.HasSuffix(pass.Fset.PositionFor(pos, false).Filename, "_test.go") {
		return
	}
	pass.Report(analysis.Diagnostic{Pos: pos, Category: "TS-C09", Message: msgReactive})
}

func insideLoop(ancestors []ast.Node) bool {
	for _, node := range ancestors {
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}
