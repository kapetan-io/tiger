// Package nogoroutine enforces TS-C02, TS-C03 and TS-C09: every goroutine
// starts through a group that waits for it, packages that start goroutines
// check for leaks in their tests, and loops don't spawn per item.
//
// TS-C02. A bare go statement is always a finding. A goroutine starts through
// errgroup.Group.Go or sync.WaitGroup.Go, and then the group's Wait must be
// reached (see internal/analyzers/internal/ownership). For a group the
// function holds, the Wait must follow the start in that function. A function
// literal, such as a t.Run body, counts as a function, and so does any
// function around it, whoever calls the literal. A literal passed to Go or
// TryGo on the same group is that group's goroutine, so neither it nor any
// literal inside it counts. A Wait after the start may also sit in a literal
// the call at that point runs before it returns (called where it is written,
// saved in a local that holds only that literal, or a t.Run body that does
// not call t.Parallel), when the literal waits on all its success paths. For
// a group held in a struct field, the Wait goes in an exported method of the
// owning type (or a method one reaches). A function counts as a Wait only
// when it reaches a call to Wait on the group, or to a function that counts,
// outside a goroutine and outside any literal it does not run: one that loops
// forever or returns only a non-nil error has no success exit to check, and
// would otherwise pass for a Wait on every group. In such a function a Wait on
// one branch, or in a defer that never runs, still counts (ENG-980). A select
// case receiving from ctx.Done() is the early return of a Shutdown whose caller gave up, so it is
// not the success path: a path through it needs no Wait, and a Wait inside it
// covers a path only when the case also holds the start. The case is exempt
// only when the path reaches it after the start, no such case holds the
// start, and the function calls Wait on the group anywhere, that case
// included. A group received as a parameter is the caller's to wait on, which
// this check does not follow. There is no allowlist of supervisor functions,
// because a function listed there would have every go statement inside it
// exempted — a per-site deviation in config shape, which ADR-0003/0005 gate.
//
// TS-C03. A package whose non-test files start goroutines through go,
// wg.Go or errgroup.Go needs a goleak check in a TestMain of its tests (see
// harness.go).
//
// TS-C09. A go statement, a sync.WaitGroup.Go call, or an errgroup.Group.Go
// or TryGo call, lexically inside a for or range loop body is the reactive
// anti-pattern TS-C09 warns about — work started in direct response to each
// event or item, with no back pressure. A group with SetLimit is still
// reported, because SetLimit caps how many goroutines run at once but not how
// many the loop starts. The rule skips _test.go files (ADR-0019): a
// concurrency test starts one goroutine per request on purpose. A spawn in a
// test loop is silent; a spawn outside a loop in a test still fires TS-C02.
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
