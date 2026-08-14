// Package paniccheck enforces TS-S18: no naked panic outside the assert
// package.
//
// One crash path, one message format, one place to change the behaviour.
// The assert package itself is the sanctioned home of panic — that is the
// rule's own exemption — and its external test package shares it, because
// proving the assert package's panic behaviour requires panicking. The
// check resolves the identifier to the builtin, so a shadowed local panic
// is not a finding.
package paniccheck

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-S18: no naked panic outside the assert package.
var Analyzer = &analysis.Analyzer{
	Name: "paniccheck",
	Doc:  "TS-S18: no naked panic outside the assert package",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() == "assert" || pass.Pkg.Name() == "assert_test" {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok || name.Name != "panic" {
				return true
			}
			if _, builtin := pass.TypesInfo.Uses[name].(*types.Builtin); !builtin {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:      call.Pos(),
				Category: "TS-S18",
				Message: "TS-S18: naked panic — route the crash through the assert package " +
					"(assert.Ok for conditions, assert.Fail for formatted failures, " +
					"assert.Unreachable for impossible arms) so there is one crash path",
			})
			return true
		})
	}
	return nil, nil
}
