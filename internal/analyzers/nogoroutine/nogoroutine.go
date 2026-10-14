// Package nogoroutine enforces TS-C02, TS-C03 and TS-C09: every goroutine
// starts through a group that waits for it, packages that start goroutines
// check for leaks in their tests, and loops don't spawn per item.
//
// TS-C02. A bare go statement is always a finding. A goroutine starts through
// errgroup.Group.Go or sync.WaitGroup.Go, and then the group's Wait must be
// reached (see internal/analyzers/internal/ownership): in the same function
// for a group the function holds, or in an exported method of the owning type
// (or a method one reaches) for a group held in a struct field. A Wait that
// sits in a select case receiving from ctx.Done() is the early return of a
// Shutdown whose caller gave up, so it is not the success path and does not
// count. A group received as a parameter is the caller's to wait on, which
// this check does not follow. There is no allowlist of supervisor functions,
// because a function listed there would have every go statement inside it
// exempted — a per-site deviation in config shape, which ADR-0003/0005 gate.
//
// TS-C03. A package whose non-test files start goroutines through go,
// wg.Go or errgroup.Go needs a goleak check in a TestMain of its tests (see
// harness.go).
//
// TS-C09. A spawn lexically inside a for or range loop body is the reactive
// anti-pattern TS-C09 warns about — work started in direct response to each
// event or item, with no back pressure. It skips _test.go files (ADR-0019): a
// concurrency test starts one goroutine per request on purpose. A spawn in a
// test loop is silent; a spawn outside a loop in a test still fires TS-C02.
package nogoroutine

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-C02, TS-C03 and TS-C09: every goroutine starts through
// a group that waits for it.
var Analyzer = &analysis.Analyzer{
	Name: "nogoroutine",
	Doc:  "TS-C02, TS-C03 and TS-C09: every goroutine starts through a group that waits for it.",
	Run:  run,
}

const (
	msgUnowned = "TS-C02: this go statement starts a goroutine nobody owns — start it with " +
		"wg.Go or errgroup.Go and Wait for the group. Shutdown shape to copy: close " +
		"a stop channel once, let the loop close done, and have every waiter select " +
		"on done"
	msgNoWaitFmt = "TS-C02: this %s starts a goroutine, but no Wait on the group is reached on " +
		"the success path — Wait before returning, or from an exported method like " +
		"Shutdown. Shape to copy: close a stop channel, close done, and have " +
		"waiters select on done"
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
				if owners.checkStart(stmt) {
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

// check reports a finding for spawn, choosing TS-C09 over TS-C02 when
// spawn sits inside a loop. A loop spawn in a _test.go file is exempt.
func check(pass *analysis.Pass, spawn *ast.GoStmt, ancestors []ast.Node) {
	if insideLoop(ancestors) {
		if strings.HasSuffix(pass.Fset.PositionFor(spawn.Pos(), false).Filename, "_test.go") {
			return
		}
		pass.Report(analysis.Diagnostic{Pos: spawn.Pos(), Category: "TS-C09", Message: msgReactive})
		return
	}
	pass.Report(analysis.Diagnostic{Pos: spawn.Pos(), Category: "TS-C02", Message: msgUnowned})
}

// insideLoop reports whether any ancestor is a for or range loop.
func insideLoop(ancestors []ast.Node) bool {
	for _, node := range ancestors {
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}
