// Package nogoroutine enforces TS-C02 and TS-C09: no bare goroutine spawns
// outside supervisors.
//
// Every GoStmt is a finding. A spawn lexically inside a for or range loop
// body is the reactive anti-pattern TS-C09 warns about — work started in
// direct response to each event or item, with no back pressure. Any other
// spawn is TS-C02: a goroutine with no documented owner and no lifetime
// tied to a context. The compliant path is errgroup.Group.Go or another
// library that owns the join; there is no allowlist of supervisor
// functions, because a function listed there would have every go statement
// inside it exempted — a per-site deviation in config shape, which
// ADR-0003/0005 gate.
package nogoroutine

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-C02 and TS-C09: no bare goroutine spawns outside
// supervisors.
var Analyzer = &analysis.Analyzer{
	Name: "nogoroutine",
	Doc:  "TS-C02 and TS-C09: no bare goroutine spawns outside supervisors.",
	Run:  run,
}

const (
	msgUnowned = "TS-C02: this go statement starts a goroutine nobody owns: nothing says when " +
		"it exits or ties it to a context — start it through errgroup.Group.Go"
	msgReactive = "TS-C09: this loop starts a goroutine per item, so the goroutines react to " +
		"each item instead of working at their own pace — append the items to a bounded " +
		"queue and drain it from one supervisor goroutine"
)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var ancestors []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				ancestors = ancestors[:len(ancestors)-1]
				return false
			}
			if spawn, ok := node.(*ast.GoStmt); ok {
				check(pass, spawn, ancestors)
			}
			ancestors = append(ancestors, node)
			return true
		})
	}
	return nil, nil
}

// check reports a finding for spawn, choosing TS-C09 over TS-C02 when
// spawn sits inside a loop.
func check(pass *analysis.Pass, spawn *ast.GoStmt, ancestors []ast.Node) {
	if insideLoop(ancestors) {
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
