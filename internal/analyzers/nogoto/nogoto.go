// Package nogoto enforces TS-S09: no goto, no labeled break or continue.
//
// Simple explicit control flow. A labeled break is usually a loop that
// wanted to be a function; a goto is control flow the reader must simulate.
// The rule is exact: a BranchStmt either carries a label (or is a goto) or
// it does not.
package nogoto

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer enforces TS-S09: no goto, no labeled break or continue.
var Analyzer = &analysis.Analyzer{
	Name: "nogoto",
	Doc:  "TS-S09: no goto, no labeled break or continue",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			branch, ok := node.(*ast.BranchStmt)
			if !ok {
				return true
			}
			switch {
			case branch.Tok == token.GOTO:
				pass.Report(analysis.Diagnostic{
					Pos:      branch.Pos(),
					Category: "TS-S09",
					Message: "TS-S09: goto transfers control invisibly — restructure " +
						"into a loop, an early return, or a function so the flow " +
						"reads top to bottom",
				})
			case branch.Label != nil:
				pass.Report(analysis.Diagnostic{
					Pos:      branch.Pos(),
					Category: "TS-S09",
					Message: "TS-S09: labeled " + branch.Tok.String() +
						" reaches across loops — extract the inner loop into a function and " +
						"return instead",
				})
			}
			return true
		})
	}
	return nil, nil
}
