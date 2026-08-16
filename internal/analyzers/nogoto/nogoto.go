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
		path := []ast.Node{}
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				path = path[:len(path)-1]
				return true
			}
			if branch, ok := node.(*ast.BranchStmt); ok {
				report(pass, branch, path)
			}
			path = append(path, node)
			return true
		})
	}
	return nil, nil
}

// report emits the TS-S09 diagnostic for a goto or a labeled branch. The
// message names what the label actually does: a break whose label marks its
// only enclosing loop exists to escape a select or switch, not to reach
// across loops, and its compliant form is the same — extract and return.
func report(pass *analysis.Pass, branch *ast.BranchStmt, path []ast.Node) {
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
		if construct, escaped := escapesSelectOrSwitch(branch, path); escaped {
			pass.Report(analysis.Diagnostic{
				Pos:      branch.Pos(),
				Category: "TS-S09",
				Message: "TS-S09: labeled " + branch.Tok.String() + " escapes a " +
					construct + " from inside its only loop — extract the loop " +
					"into a function and return instead",
			})
			return
		}
		pass.Report(analysis.Diagnostic{
			Pos:      branch.Pos(),
			Category: "TS-S09",
			Message: "TS-S09: labeled " + branch.Tok.String() +
				" reaches across loops — extract the inner loop into a function and " +
				"return instead",
		})
	}
}

// escapesSelectOrSwitch decides whether a labeled break targets its only
// enclosing loop from inside a select or switch — the shape Go forces a
// label onto because a bare break would leave just the select or switch.
// It returns the innermost such construct's name when so.
func escapesSelectOrSwitch(branch *ast.BranchStmt, path []ast.Node) (string, bool) {
	if branch.Tok != token.BREAK {
		return "", false
	}
	construct := ""
	loops := 0
	for i := len(path) - 1; i >= 0; i-- {
		switch enclosing := path[i].(type) {
		case *ast.SelectStmt:
			if construct == "" {
				construct = "select"
			}
		case *ast.SwitchStmt, *ast.TypeSwitchStmt:
			if construct == "" {
				construct = "switch"
			}
		case *ast.ForStmt, *ast.RangeStmt:
			loops++
		case *ast.LabeledStmt:
			if enclosing.Label.Name == branch.Label.Name {
				return construct, construct != "" && loops == 1
			}
		case *ast.FuncDecl, *ast.FuncLit:
			return "", false
		}
	}
	return "", false
}
