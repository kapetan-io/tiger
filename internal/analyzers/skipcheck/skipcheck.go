// Package skipcheck enforces TS-D07: skipped tests are reported on every run.
//
// A call to Skip, Skipf, or SkipNow on a *testing.T, *testing.B, *testing.F,
// or testing.TB receiver surfaces as a standing advisory finding — the same
// never-silent mechanism escapes use. Visibility beats paperwork: an earlier
// form of the rule demanded a //tiger:skip directive with a tracker ID, but a
// tracker reference can outlive its issue, while a report recomputed from the
// code on every run cannot go stale.
package skipcheck

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-D07: skipped tests are reported on every run.
var Analyzer = &analysis.Analyzer{
	Name: "skipcheck",
	Doc:  "TS-D07: skipped tests are reported on every run",
	Run:  run,
}

// skipMethods names the testing methods the report covers.
var skipMethods = map[string]bool{"Skip": true, "Skipf": true, "SkipNow": true}

// testingTypes names the testing receiver types the report covers.
var testingTypes = map[string]bool{"T": true, "B": true, "F": true, "TB": true}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			inspectCall(pass, node)
			return true
		})
	}
	return nil, nil
}

// inspectCall reports node when it is a testing skip call.
func inspectCall(pass *analysis.Pass, node ast.Node) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !skipMethods[selector.Sel.Name] {
		return
	}
	if !isTestingReceiver(pass.TypesInfo.TypeOf(selector.X)) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      call.Pos(),
		Category: "TS-D07",
		Message: "TS-D07: skipped test — a skipped test is a test that passes; " +
			"this notice stands until the " + selector.Sel.Name + " call is removed",
	})
}

// isTestingReceiver reports whether target is *testing.T, *testing.B,
// *testing.F, or testing.TB.
func isTestingReceiver(target types.Type) bool {
	if target == nil {
		return false
	}
	if pointer, ok := target.(*types.Pointer); ok {
		target = pointer.Elem()
	}
	named, ok := target.(*types.Named)
	if !ok {
		return false
	}
	symbol := named.Obj()
	if symbol.Pkg() == nil || symbol.Pkg().Path() != "testing" {
		return false
	}
	return testingTypes[symbol.Name()]
}
