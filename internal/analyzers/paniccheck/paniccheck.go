// Package paniccheck enforces TS-S18: no naked panic outside the assert
// package.
package paniccheck

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/assertpkg"
)

var Analyzer = &analysis.Analyzer{
	Name: "paniccheck",
	Doc:  "TS-S18: no naked panic outside the assert package",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if assertpkg.Is(pass.Pkg.Path()) {
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
				Message: "TS-S18: panic is called directly here — use assert.Ok (condition), " +
					"assert.Fail (formatted failure), or assert.Unreachable (impossible arm) " +
					"instead so every crash goes through one path",
			})
			return true
		})
	}
	return nil, nil
}
